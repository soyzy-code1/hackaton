package main_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"invoicekeeper/internal/config"
	"invoicekeeper/internal/dates"
	"invoicekeeper/internal/db"
	"invoicekeeper/internal/handlers"
	"invoicekeeper/internal/service"
	"invoicekeeper/internal/store"
)

func testRouter(t *testing.T) (*gin.Engine, *store.Store) {
	os.Setenv("SECRET_KEY", "test-secret")
	os.Setenv("INTERNAL_API_TOKEN", "test-internal-token")
	os.Setenv("ADMIN_LOGIN", "admin")
	os.Setenv("ADMIN_PASSWORD", "admin123")
	os.Setenv("ADMIN_EMAIL", "admin@test.com")
	os.Setenv("TIMEZONE", "Europe/Moscow")
	os.Setenv("REMINDER_DAYS_BEFORE", "3")
	cfg := config.Load()
	database, err := db.Open("sqlite:///:memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatal(err)
	}
	st := &store.Store{DB: database}
	if err := service.EnsureAdmin(st, cfg); err != nil {
		t.Fatal(err)
	}
	return handlers.NewRouter(cfg, st), st
}

func login(t *testing.T, r *gin.Engine, login, pass string) string {
	body := `{"login":"` + login + `","password":"` + pass + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("login %s: %d %s", login, w.Code, w.Body.String())
	}
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "access_token" {
			return cookie.Value
		}
	}
	t.Fatalf("login %s: access_token cookie missing: %s", login, w.Body.String())
	return ""
}

func TestAuthFlow(t *testing.T) {
	r, _ := testRouter(t)
	token := login(t, r, "admin", "admin123")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	req2.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != 204 {
		t.Fatal(w2.Code)
	}
	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req3.Header.Set("Authorization", "Bearer "+token)
	req3.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	if w3.Code != 401 {
		t.Fatal(w3.Code)
	}
}

func TestMonthClamp(t *testing.T) {
	r, _ := testRouter(t)
	token := login(t, r, "admin", "admin123")
	create := `{"title":"Rent","amount":"1000","currency":"RUB","next_payment_date":"2026-01-31"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/recurring-payments", bytes.NewBufferString(create))
	req.Header.Set("Authorization", "Bearer "+token)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var payment map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &payment)
	if w.Code != http.StatusCreated {
		t.Fatalf("create payment: %d %s", w.Code, w.Body.String())
	}
	pid := int(payment["id"].(float64))

	reqL := httptest.NewRequest(http.MethodGet, "/api/v1/payment-occurrences?recurring_payment_id="+itoa(pid), nil)
	reqL.Header.Set("Authorization", "Bearer "+token)
	reqL.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	wL := httptest.NewRecorder()
	r.ServeHTTP(wL, reqL)
	var occs []map[string]interface{}
	_ = json.Unmarshal(wL.Body.Bytes(), &occs)
	oid := int(occs[0]["id"].(float64))

	patch := `{"is_paid":true,"paid_at":"2026-01-31"}`
	reqP := httptest.NewRequest(http.MethodPatch, "/api/v1/payment-occurrences/"+itoa(oid), bytes.NewBufferString(patch))
	reqP.Header.Set("Authorization", "Bearer "+token)
	reqP.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	reqP.Header.Set("Content-Type", "application/json")
	wP := httptest.NewRecorder()
	r.ServeHTTP(wP, reqP)

	reqL2 := httptest.NewRequest(http.MethodGet, "/api/v1/payment-occurrences?recurring_payment_id="+itoa(pid), nil)
	reqL2.Header.Set("Authorization", "Bearer "+token)
	reqL2.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	wL2 := httptest.NewRecorder()
	r.ServeHTTP(wL2, reqL2)
	_ = json.Unmarshal(wL2.Body.Bytes(), &occs)
	datesList := []string{occs[0]["due_date"].(string), occs[1]["due_date"].(string)}
	if datesList[0] != "2026-01-31" || datesList[1] != "2026-02-28" {
		t.Fatalf("dates %v", datesList)
	}
}

func TestInternalDue(t *testing.T) {
	r, _ := testRouter(t)
	token := login(t, r, "admin", "admin123")
	today := dates.TodayInTZ("Europe/Moscow")
	due := today.AddDate(0, 0, 2)
	create := `{"title":"Reminder","amount":"10","currency":"RUB","next_payment_date":"` + dates.FormatDate(due) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/recurring-payments", bytes.NewBufferString(create))
	req.Header.Set("Authorization", "Bearer "+token)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	reqD := httptest.NewRequest(http.MethodGet, "/api/v1/internal/due-occurrences", nil)
	reqD.Header.Set("X-Internal-Token", "test-internal-token")
	wD := httptest.NewRecorder()
	r.ServeHTTP(wD, reqD)
	if wD.Code != 200 {
		t.Fatal(wD.Body.String())
	}
	var items []map[string]interface{}
	_ = json.Unmarshal(wD.Body.Bytes(), &items)
	if len(items) < 1 {
		t.Fatal("expected due item")
	}
	oid := int(items[0]["id"].(float64))
	reqM := httptest.NewRequest(http.MethodPost, "/api/v1/internal/occurrences/"+itoa(oid)+"/mark-reminded", nil)
	reqM.Header.Set("X-Internal-Token", "test-internal-token")
	wM := httptest.NewRecorder()
	r.ServeHTTP(wM, reqM)
	if wM.Code != 204 {
		t.Fatal(wM.Code)
	}
}

// TestInternalDueIncludesOverdue pins the due-occurrence semantics shared by the
// backend and the bot.
//
// The backend used to filter `due_date >= today`, which hid every overdue
// payment from the email, webhook and internal reminder channels while the bot
// (bot/api_client.py) happily reported the same payments. The rule is now
// "unpaid and due no later than the horizon", so overdue occurrences must be
// returned as well.
func TestInternalDueIncludesOverdue(t *testing.T) {
	r, _ := testRouter(t)
	token := login(t, r, "admin", "admin123")
	today := dates.TodayInTZ("Europe/Moscow")

	overdue := today.AddDate(0, 0, -5)
	upcoming := today.AddDate(0, 0, 2)
	tooLate := today.AddDate(0, 0, 30)

	for _, tc := range []struct {
		title string
		due   time.Time
	}{
		{"Overdue bill", overdue},
		{"Upcoming bill", upcoming},
		{"Distant bill", tooLate},
	} {
		create := `{"title":"` + tc.title + `","amount":"10","currency":"RUB","next_payment_date":"` + dates.FormatDate(tc.due) + `"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/recurring-payments", bytes.NewBufferString(create))
		req.Header.Set("Authorization", "Bearer "+token)
		req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("create %s: %d %s", tc.title, w.Code, w.Body.String())
		}
	}

	reqD := httptest.NewRequest(http.MethodGet, "/api/v1/internal/due-occurrences", nil)
	reqD.Header.Set("X-Internal-Token", "test-internal-token")
	wD := httptest.NewRecorder()
	r.ServeHTTP(wD, reqD)
	if wD.Code != http.StatusOK {
		t.Fatalf("due occurrences: %d %s", wD.Code, wD.Body.String())
	}

	var items []map[string]interface{}
	if err := json.Unmarshal(wD.Body.Bytes(), &items); err != nil {
		t.Fatal(err)
	}

	got := map[string]int{}
	for _, item := range items {
		title, _ := item["title"].(string)
		got[title] = int(item["days_left"].(float64))
	}

	if _, ok := got["Overdue bill"]; !ok {
		t.Fatalf("overdue occurrence is missing from due list: %+v", items)
	}
	if _, ok := got["Upcoming bill"]; !ok {
		t.Fatalf("upcoming occurrence is missing from due list: %+v", items)
	}
	if _, ok := got["Distant bill"]; ok {
		t.Fatalf("occurrence beyond the reminder horizon must not be listed: %+v", items)
	}
	if days := got["Overdue bill"]; days >= 0 {
		t.Fatalf("overdue occurrence must report negative days_left, got %d", days)
	}
}

// TestInternalDueSkipsReminded guards the deduplication of the reminder
// channel: an occurrence that was already marked as reminded must not be
// returned again, and marking it twice must stay idempotent.
func TestInternalDueSkipsReminded(t *testing.T) {
	r, _ := testRouter(t)
	token := login(t, r, "admin", "admin123")
	today := dates.TodayInTZ("Europe/Moscow")
	due := today.AddDate(0, 0, -1)

	create := `{"title":"Reminded bill","amount":"10","currency":"RUB","next_payment_date":"` + dates.FormatDate(due) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/recurring-payments", bytes.NewBufferString(create))
	req.Header.Set("Authorization", "Bearer "+token)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}

	listDue := func() []map[string]interface{} {
		t.Helper()
		reqD := httptest.NewRequest(http.MethodGet, "/api/v1/internal/due-occurrences", nil)
		reqD.Header.Set("X-Internal-Token", "test-internal-token")
		wD := httptest.NewRecorder()
		r.ServeHTTP(wD, reqD)
		if wD.Code != http.StatusOK {
			t.Fatalf("due occurrences: %d %s", wD.Code, wD.Body.String())
		}
		var items []map[string]interface{}
		if err := json.Unmarshal(wD.Body.Bytes(), &items); err != nil {
			t.Fatal(err)
		}
		return items
	}

	items := listDue()
	if len(items) != 1 {
		t.Fatalf("expected exactly one due occurrence, got %d: %+v", len(items), items)
	}
	oid := int(items[0]["id"].(float64))

	mark := httptest.NewRequest(http.MethodPost, "/api/v1/internal/occurrences/"+itoa(oid)+"/mark-reminded", nil)
	mark.Header.Set("X-Internal-Token", "test-internal-token")
	wM := httptest.NewRecorder()
	r.ServeHTTP(wM, mark)
	if wM.Code != http.StatusNoContent {
		t.Fatalf("mark reminded: %d %s", wM.Code, wM.Body.String())
	}

	if items = listDue(); len(items) != 0 {
		t.Fatalf("reminded occurrence must not be listed again: %+v", items)
	}

	repeat := httptest.NewRequest(http.MethodPost, "/api/v1/internal/occurrences/"+itoa(oid)+"/mark-reminded", nil)
	repeat.Header.Set("X-Internal-Token", "test-internal-token")
	wR := httptest.NewRecorder()
	r.ServeHTTP(wR, repeat)
	if wR.Code != http.StatusOK {
		t.Fatalf("repeat mark reminded: %d %s", wR.Code, wR.Body.String())
	}
}

// TestInternalDueRequiresToken makes sure the internal endpoint stays closed
// to anonymous callers: the bot authenticates with X-Internal-Token.
func TestInternalDueRequiresToken(t *testing.T) {
	r, _ := testRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/internal/due-occurrences", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without internal token, got %d", w.Code)
	}
}

func TestFrontendMinimalContract(t *testing.T) {
	r, _ := testRouter(t)
	token := login(t, r, "admin", "admin123")
	create := `{"title":"Подписка","amount":"99.00","currency":"EUR","next_payment_date":"2026-12-01"}`
	reqC := httptest.NewRequest(http.MethodPost, "/api/v1/recurring-payments", bytes.NewBufferString(create))
	reqC.Header.Set("Authorization", "Bearer "+token)
	reqC.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	reqC.Header.Set("Content-Type", "application/json")
	wC := httptest.NewRecorder()
	r.ServeHTTP(wC, reqC)

	reqG := httptest.NewRequest(http.MethodGet, "/api/v1/payment-occurrences?view=minimal", nil)
	reqG.Header.Set("Authorization", "Bearer "+token)
	reqG.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	wG := httptest.NewRecorder()
	r.ServeHTTP(wG, reqG)
	if wG.Code != 200 {
		t.Fatal(wG.Body.String())
	}
	var list []map[string]interface{}
	_ = json.Unmarshal(wG.Body.Bytes(), &list)
	if len(list) == 0 {
		t.Fatal("empty list")
	}
	row := list[0]
	for _, key := range []string{"id", "title", "amount", "currency", "due_date", "is_paid", "actual_payment_date"} {
		if _, ok := row[key]; !ok {
			t.Fatalf("missing key %s", key)
		}
	}
	if _, ok := row["is_overdue"]; ok {
		t.Fatal("minimal must not include is_overdue")
	}
	oid := int(row["id"].(float64))
	paidDay := dates.FormatDate(dates.TodayInTZ("Europe/Moscow"))
	patch := `{"is_paid":true,"actual_payment_date":"` + paidDay + `"}`
	reqP := httptest.NewRequest(http.MethodPatch, "/api/v1/payment-occurrences/"+itoa(oid)+"?view=minimal", bytes.NewBufferString(patch))
	reqP.Header.Set("Authorization", "Bearer "+token)
	reqP.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	reqP.Header.Set("Content-Type", "application/json")
	wP := httptest.NewRecorder()
	r.ServeHTTP(wP, reqP)
	if wP.Code != 200 {
		t.Fatal(wP.Body.String())
	}
	var one map[string]interface{}
	_ = json.Unmarshal(wP.Body.Bytes(), &one)
	if one["is_paid"] != true {
		t.Fatal("expected paid")
	}
	if one["actual_payment_date"] != paidDay {
		t.Fatalf("date %v", one["actual_payment_date"])
	}
}

func TestAuditLogAccessAndPagination(t *testing.T) {
	r, st := testRouter(t)
	token := login(t, r, "admin", "admin123")

	request := httptest.NewRequest(http.MethodGet, "/api/v1/audit-log?page=1&page_size=10&action=login", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	request.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	response := httptest.NewRecorder()
	r.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("audit log status: %d %s", response.Code, response.Body.String())
	}

	var payload struct {
		Items    []map[string]interface{} `json:"items"`
		Total    int                      `json:"total"`
		Page     int                      `json:"page"`
		PageSize int                      `json:"page_size"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Total < 1 || len(payload.Items) < 1 || payload.Page != 1 || payload.PageSize != 10 {
		t.Fatalf("unexpected audit payload: %+v", payload)
	}
	if payload.Items[0]["action"] != "login" {
		t.Fatalf("unexpected audit action: %+v", payload.Items[0])
	}

	user, err := st.GetUserByLogin("admin")
	if err != nil || user == nil {
		t.Fatalf("admin lookup: %v", err)
	}
	_ = user

	unauthorized := httptest.NewRequest(http.MethodGet, "/api/v1/audit-log", nil)
	unauthorizedResponse := httptest.NewRecorder()
	r.ServeHTTP(unauthorizedResponse, unauthorized)
	if unauthorizedResponse.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized audit log status: %d", unauthorizedResponse.Code)
	}
}

func TestGigaChatUserAccessAndProxy(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth":
			if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Basic test-credentials" {
				http.Error(w, "unexpected oauth request", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"test-access-token","expires_at":4102444800000}`))
		case "/api/v1/chat/completions":
			if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer test-access-token" {
				http.Error(w, "unexpected completion request", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"Тестовый ответ GigaChat"}}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer provider.Close()

	for key, value := range map[string]string{
		"GIGACHAT_ENABLED":     "true",
		"GIGACHAT_CREDENTIALS": "test-credentials",
		"GIGACHAT_AUTH_URL":    provider.URL + "/oauth",
		"GIGACHAT_API_URL":     provider.URL + "/api/v1",
		"GIGACHAT_MODEL":       "GigaChat",
		"GIGACHAT_VERIFY_TLS":  "true",
	} {
		if err := os.Setenv(key, value); err != nil {
			t.Fatal(err)
		}
	}
	defer func() {
		for _, key := range []string{"GIGACHAT_ENABLED", "GIGACHAT_CREDENTIALS", "GIGACHAT_AUTH_URL", "GIGACHAT_API_URL", "GIGACHAT_MODEL", "GIGACHAT_VERIFY_TLS"} {
			_ = os.Unsetenv(key)
		}
	}()

	r, _ := testRouter(t)
	adminToken := login(t, r, "admin", "admin123")

	createUser := `{"login":"assistant-user","password":"user123"}`
	createRequest := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewBufferString(createUser))
	createRequest.Header.Set("Authorization", "Bearer "+adminToken)
	createRequest.AddCookie(&http.Cookie{Name: "access_token", Value: adminToken})
	createRequest.Header.Set("Content-Type", "application/json")
	createResponse := httptest.NewRecorder()
	r.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create user: %d %s", createResponse.Code, createResponse.Body.String())
	}
	userToken := login(t, r, "assistant-user", "user123")

	adminRequest := httptest.NewRequest(http.MethodPost, "/api/v1/chat/gigachat", bytes.NewBufferString(`{"message":"admin access"}`))
	adminRequest.Header.Set("Authorization", "Bearer "+adminToken)
	adminRequest.AddCookie(&http.Cookie{Name: "access_token", Value: adminToken})
	adminResponse := httptest.NewRecorder()
	r.ServeHTTP(adminResponse, adminRequest)
	if adminResponse.Code != http.StatusForbidden {
		t.Fatalf("admin chat access: %d %s", adminResponse.Code, adminResponse.Body.String())
	}

	userRequest := httptest.NewRequest(http.MethodPost, "/api/v1/chat/gigachat", bytes.NewBufferString(`{"message":"Как вести бюджет?"}`))
	userRequest.Header.Set("Authorization", "Bearer "+userToken)
	userRequest.AddCookie(&http.Cookie{Name: "access_token", Value: userToken})
	userRequest.Header.Set("Content-Type", "application/json")
	userResponse := httptest.NewRecorder()
	r.ServeHTTP(userResponse, userRequest)
	if userResponse.Code != http.StatusOK || !bytes.Contains(userResponse.Body.Bytes(), []byte("Тестовый ответ GigaChat")) {
		t.Fatalf("user chat: %d %s", userResponse.Code, userResponse.Body.String())
	}

	unauthorizedRequest := httptest.NewRequest(http.MethodPost, "/api/v1/chat/gigachat", bytes.NewBufferString(`{"message":"anonymous"}`))
	unauthorizedResponse := httptest.NewRecorder()
	r.ServeHTTP(unauthorizedResponse, unauthorizedRequest)
	if unauthorizedResponse.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized chat access: %d %s", unauthorizedResponse.Code, unauthorizedResponse.Body.String())
	}
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
