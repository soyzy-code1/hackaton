package main_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"

	"invoicekeeper/internal/config"
	"invoicekeeper/internal/db"
	"invoicekeeper/internal/dates"
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
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	return resp["access_token"].(string)
}

func TestAuthFlow(t *testing.T) {
	r, _ := testRouter(t)
	token := login(t, r, "admin", "admin123")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != 204 {
		t.Fatal(w2.Code)
	}
	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req3.Header.Set("Authorization", "Bearer "+token)
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
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var payment map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &payment)
	pid := int(payment["id"].(float64))

	reqL := httptest.NewRequest(http.MethodGet, "/api/v1/payment-occurrences?recurring_payment_id="+itoa(pid), nil)
	reqL.Header.Set("Authorization", "Bearer "+token)
	wL := httptest.NewRecorder()
	r.ServeHTTP(wL, reqL)
	var occs []map[string]interface{}
	_ = json.Unmarshal(wL.Body.Bytes(), &occs)
	oid := int(occs[0]["id"].(float64))

	patch := `{"is_paid":true,"paid_at":"2026-01-31"}`
	reqP := httptest.NewRequest(http.MethodPatch, "/api/v1/payment-occurrences/"+itoa(oid), bytes.NewBufferString(patch))
	reqP.Header.Set("Authorization", "Bearer "+token)
	reqP.Header.Set("Content-Type", "application/json")
	wP := httptest.NewRecorder()
	r.ServeHTTP(wP, reqP)

	reqL2 := httptest.NewRequest(http.MethodGet, "/api/v1/payment-occurrences?recurring_payment_id="+itoa(pid), nil)
	reqL2.Header.Set("Authorization", "Bearer "+token)
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

func TestFrontendMinimalContract(t *testing.T) {
	r, _ := testRouter(t)
	token := login(t, r, "admin", "admin123")
	create := `{"title":"Подписка","amount":"99.00","currency":"EUR","next_payment_date":"2026-12-01"}`
	reqC := httptest.NewRequest(http.MethodPost, "/api/v1/recurring-payments", bytes.NewBufferString(create))
	reqC.Header.Set("Authorization", "Bearer "+token)
	reqC.Header.Set("Content-Type", "application/json")
	wC := httptest.NewRecorder()
	r.ServeHTTP(wC, reqC)

	reqG := httptest.NewRequest(http.MethodGet, "/api/v1/payment-occurrences?view=minimal", nil)
	reqG.Header.Set("Authorization", "Bearer "+token)
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

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
