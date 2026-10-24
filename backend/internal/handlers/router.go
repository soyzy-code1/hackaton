package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"invoicekeeper/internal/apperr"
	"invoicekeeper/internal/auth"
	"invoicekeeper/internal/config"
	"invoicekeeper/internal/dates"
	"invoicekeeper/internal/models"
	"invoicekeeper/internal/notify"
	"invoicekeeper/internal/service"
	"invoicekeeper/internal/store"
)

type Server struct {
	Cfg config.Config
	St  *store.Store
}

func NewRouter(cfg config.Config, st *store.Store) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	s := &Server{Cfg: cfg, St: st}
	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/api/v1/openapi.json", s.openAPI)
	r.GET("/api/v1/docs", s.swaggerUI)

	v1 := r.Group("/api/v1")
	v1.POST("/auth/login", s.login)
	v1.POST("/auth/logout", authMiddleware(cfg, st), s.logout)
	v1.GET("/auth/me", authMiddleware(cfg, st), s.me)

	users := v1.Group("/users", authMiddleware(cfg, st), requireAdmin)
	users.GET("", s.listUsers)
	users.POST("", s.createUser)
	users.PATCH("/:id", s.updateUser)
	users.DELETE("/:id", s.deleteUser)

	payments := v1.Group("/recurring-payments", authMiddleware(cfg, st))
	payments.GET("", s.listRecurringPayments)
	payments.POST("", requireAdmin, s.createRecurringPayment)
	payments.DELETE("/:id", requireAdmin, s.deleteRecurringPayment)

	occ := v1.Group("/payment-occurrences", authMiddleware(cfg, st))
	occ.GET("", s.listOccurrences)
	occ.GET("/:id", s.getOccurrence)
	occ.PATCH("/:id", s.patchOccurrence)

	v1.GET("/payments/summary", authMiddleware(cfg, st), s.summary)

	internal := v1.Group("/internal", internalTokenMiddleware(cfg))
	internal.GET("/due-occurrences", s.dueOccurrences)
	internal.POST("/occurrences/:id/mark-reminded", s.markReminded)

	return r
}

func (s *Server) login(c *gin.Context) {
	var body struct {
		// Login — canonical backend field. Username is accepted as a backwards-
		// compatible alias used by the current frontend contract.
		Login    string `json:"login"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeAppError(c, apperr.ValidationError)
		return
	}
	login := strings.TrimSpace(body.Login)
	if login == "" {
		login = strings.TrimSpace(body.Username)
	}
	if login == "" || body.Password == "" {
		writeAppError(c, apperr.ValidationError)
		return
	}
	user, err := s.St.GetUserByLogin(login)
	if err != nil || user == nil || !user.IsActive || !auth.VerifyPassword(body.Password, user.PasswordHash) {
		writeAppError(c, apperr.New(http.StatusUnauthorized, "Invalid credentials", "AUTH_REQUIRED"))
		return
	}
	token, _, _, err := auth.CreateToken(user.ID, s.Cfg.SecretKey, s.Cfg.SessionTTLHours)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	// Same-origin frontend uses this HttpOnly cookie on page reloads.
	// The JSON token remains for API clients/backwards compatibility.
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "invoice_keeper_token",
		Value:    token,
		Path:     "/",
		MaxAge:   s.Cfg.SessionTTLHours * 60 * 60,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   c.Request.TLS != nil,
	})
	c.JSON(http.StatusOK, gin.H{
		"access_token": token,
		"token_type":   "bearer",
		"user":         toUserOut(user),
	})
}

func (s *Server) me(c *gin.Context) {
	u, _ := currentUser(c)
	c.JSON(http.StatusOK, toUserOut(u))
}

func (s *Server) logout(c *gin.Context) {
	token := ""
	h := c.GetHeader("Authorization")
	if h != "" && strings.HasPrefix(strings.ToLower(h), "bearer ") {
		token = strings.TrimSpace(h[7:])
	} else if cookie, err := c.Request.Cookie("invoice_keeper_token"); err == nil {
		token = strings.TrimSpace(cookie.Value)
	}
	if token == "" {
		writeAppError(c, apperr.AuthRequired)
		return
	}
	claims, err := auth.ParseToken(token, s.Cfg.SecretKey)
	if err != nil {
		writeAppError(c, apperr.New(http.StatusUnauthorized, "Invalid token", "AUTH_REQUIRED"))
		return
	}
	if claims.JTI != "" {
		exp := time.Now().UTC()
		if claims.ExpiresAt != nil {
			exp = claims.ExpiresAt.Time
		}
		_ = s.St.RevokeToken(claims.JTI, exp)
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name: "invoice_keeper_token", Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: c.Request.TLS != nil,
	})
	c.Status(http.StatusNoContent)
}

func (s *Server) listUsers(c *gin.Context) {
	users, err := s.St.ListUsers()
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	out := make([]userOut, 0, len(users))
	for _, u := range users {
		out = append(out, toUserOut(&u))
	}
	c.JSON(http.StatusOK, out)
}

func (s *Server) createUser(c *gin.Context) {
	var body struct {
		Login    string  `json:"login"`
		Password string  `json:"password"`
		Email    *string `json:"email"`
		FullName *string `json:"full_name"`
		Role     *string `json:"role"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeAppError(c, apperr.ValidationError)
		return
	}
	if body.Role != nil && *body.Role != "user" {
		writeAppError(c, apperr.New(http.StatusUnprocessableEntity, "Cannot assign admin role", "VALIDATION_ERROR"))
		return
	}
	existing, _ := s.St.GetUserByLogin(body.Login)
	if existing != nil {
		writeAppError(c, apperr.New(http.StatusConflict, "Login already exists", "CONFLICT"))
		return
	}
	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	u := &models.User{
		Login: body.Login, Email: body.Email, FullName: body.FullName,
		PasswordHash: hash, Role: "user", IsActive: true,
	}
	if err := s.St.CreateUser(u); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusCreated, toUserOut(u))
}

func (s *Server) updateUser(c *gin.Context) {
	var body struct {
		Email    *string `json:"email"`
		FullName *string `json:"full_name"`
		Password *string `json:"password"`
		IsActive *bool   `json:"is_active"`
		Role     *string `json:"role"`
		Login    *string `json:"login"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeAppError(c, apperr.ValidationError)
		return
	}
	if body.Role != nil || body.Login != nil {
		writeAppError(c, apperr.New(http.StatusUnprocessableEntity, "Cannot change role or login", "VALIDATION_ERROR"))
		return
	}
	id, _ := strconvAtoi(c.Param("id"))
	user, err := s.St.GetUserByID(id)
	if err != nil || user == nil {
		writeAppError(c, apperr.New(http.StatusNotFound, "User not found", "NOT_FOUND"))
		return
	}
	if body.Email != nil {
		user.Email = body.Email
	}
	if body.FullName != nil {
		user.FullName = body.FullName
	}
	if body.Password != nil {
		hash, err := auth.HashPassword(*body.Password)
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		user.PasswordHash = hash
	}
	if body.IsActive != nil {
		user.IsActive = *body.IsActive
	}
	if err := s.St.UpdateUser(user); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, toUserOut(user))
}

func (s *Server) deleteUser(c *gin.Context) {
	id, _ := strconvAtoi(c.Param("id"))
	user, err := s.St.GetUserByID(id)
	if err != nil || user == nil {
		writeAppError(c, apperr.New(http.StatusNotFound, "User not found", "NOT_FOUND"))
		return
	}
	if user.Role == "admin" {
		writeAppError(c, apperr.New(http.StatusForbidden, "Cannot delete admin user", "FORBIDDEN"))
		return
	}
	if err := s.St.DeleteUser(id); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) listRecurringPayments(c *gin.Context) {
	var currency *string
	if v := c.Query("currency"); v != "" {
		currency = &v
	}
	var isActive *bool
	if v := c.Query("is_active"); v != "" {
		b := v == "true"
		isActive = &b
	}
	items, err := s.St.ListRecurringPayments(currency, isActive)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	out := make([]recurringPaymentOut, 0, len(items))
	for _, p := range items {
		out = append(out, toPaymentOut(&p))
	}
	c.JSON(http.StatusOK, out)
}

func (s *Server) createRecurringPayment(c *gin.Context) {
	var body struct {
		Title           string          `json:"title"`
		Amount          decimal.Decimal `json:"amount"`
		Currency        string          `json:"currency"`
		NextPaymentDate string          `json:"next_payment_date"`
		Periodicity     string          `json:"periodicity"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeAppError(c, apperr.ValidationError)
		return
	}
	if body.Amount.LessThanOrEqual(decimal.Zero) {
		writeAppError(c, apperr.ValidationError)
		return
	}
	if body.Periodicity == "" {
		body.Periodicity = "monthly"
	}
	nd, err := dates.ParseDate(body.NextPaymentDate)
	if err != nil {
		writeAppError(c, apperr.ValidationError)
		return
	}
	admin, _ := currentUser(c)
	p := &models.RecurringPayment{
		Title: body.Title, Amount: body.Amount, Currency: body.Currency,
		Periodicity: body.Periodicity, NextPaymentDate: nd, IsActive: true, CreatedBy: admin.ID,
	}
	if err := s.St.CreateRecurringPayment(p); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	_, err = service.EnsureNextOccurrence(s.St, s.Cfg, p)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	notify.PostWebhookAsync(s.Cfg, notify.EventRecurringPaymentCreated, notify.RecurringPaymentData{
		ID: p.ID, Title: p.Title, Amount: p.Amount.StringFixed(2),
		Currency: p.Currency, NextPaymentDate: dates.FormatDate(p.NextPaymentDate),
	})
	c.JSON(http.StatusCreated, toPaymentOut(p))
}

func (s *Server) deleteRecurringPayment(c *gin.Context) {
	id, _ := strconvAtoi(c.Param("id"))
	p, err := s.St.GetRecurringPayment(id)
	if err != nil || p == nil || p.DeletedAt != nil {
		writeAppError(c, apperr.New(http.StatusNotFound, "Payment not found", "NOT_FOUND"))
		return
	}
	notify.PostWebhookAsync(s.Cfg, notify.EventRecurringPaymentDeleted, notify.RecurringPaymentData{
		ID: p.ID, Title: p.Title, Amount: p.Amount.StringFixed(2),
		Currency: p.Currency, NextPaymentDate: dates.FormatDate(p.NextPaymentDate),
	})
	if err := s.St.SoftDeletePayment(id); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) listOccurrences(c *gin.Context) {
	if err := service.LazyEnsureActive(s.St, s.Cfg); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	var currency *string
	if v := c.Query("currency"); v != "" {
		currency = &v
	}
	var rid *int
	if v := c.Query("recurring_payment_id"); v != "" {
		id, _ := strconvAtoi(v)
		rid = &id
	}
	var from, to *time.Time
	if v := c.Query("from_date"); v != "" {
		t, err := dates.ParseDate(v)
		if err == nil {
			from = &t
		}
	}
	if v := c.Query("to_date"); v != "" {
		t, err := dates.ParseDate(v)
		if err == nil {
			to = &t
		}
	}
	rows, err := s.St.ListOccurrences(currency, rid, from, to)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	status := c.Query("status")
	minimalView := c.Query("view") == "minimal"
	tz := s.Cfg.Timezone
	if minimalView {
		minimal := make([]occurrenceMinimalOut, 0, len(rows))
		for _, o := range rows {
			item := toOccurrenceOut(&o, tz)
			if !occurrenceMatchesStatus(item, status) {
				continue
			}
			minimal = append(minimal, toOccurrenceMinimal(&o, tz))
		}
		c.JSON(http.StatusOK, minimal)
		return
	}
	out := make([]occurrenceOut, 0, len(rows))
	for _, o := range rows {
		item := toOccurrenceOut(&o, tz)
		if !occurrenceMatchesStatus(item, status) {
			continue
		}
		out = append(out, item)
	}
	c.JSON(http.StatusOK, out)
}

func occurrenceMatchesStatus(item occurrenceOut, status string) bool {
	switch status {
	case "paid":
		return item.IsPaid
	case "unpaid":
		return !item.IsPaid && !item.IsOverdue
	case "overdue":
		return item.IsOverdue
	default:
		return true
	}
}

func (s *Server) getOccurrence(c *gin.Context) {
	id, _ := strconvAtoi(c.Param("id"))
	o, err := s.St.GetOccurrence(id)
	if err != nil || o == nil {
		writeAppError(c, apperr.New(http.StatusNotFound, "Occurrence not found", "NOT_FOUND"))
		return
	}
	s.writeOccurrenceJSON(c, o)
}

func (s *Server) writeOccurrenceJSON(c *gin.Context, o *models.PaymentOccurrence) {
	if c.Query("view") == "minimal" {
		c.JSON(http.StatusOK, toOccurrenceMinimal(o, s.Cfg.Timezone))
		return
	}
	c.JSON(http.StatusOK, toOccurrenceOut(o, s.Cfg.Timezone))
}

func (s *Server) patchOccurrence(c *gin.Context) {
	id, _ := strconvAtoi(c.Param("id"))
	o, err := s.St.GetOccurrence(id)
	if err != nil || o == nil {
		writeAppError(c, apperr.New(http.StatusNotFound, "Occurrence not found", "NOT_FOUND"))
		return
	}
	var body struct {
		IsPaid              bool    `json:"is_paid"`
		PaidAt              *string `json:"paid_at"`
		ActualPaymentDate   *string `json:"actual_payment_date"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeAppError(c, apperr.ValidationError)
		return
	}
	if body.PaidAt == nil && body.ActualPaymentDate != nil {
		body.PaidAt = body.ActualPaymentDate
	}
	user, _ := currentUser(c)
	today := dates.TodayInTZ(s.Cfg.Timezone)
	wasPaid := o.IsPaid

	if body.IsPaid {
		var paidAt time.Time
		if body.PaidAt != nil {
			paidAt, err = dates.ParseDate(*body.PaidAt)
			if err != nil {
				writeAppError(c, apperr.ValidationError)
				return
			}
		} else if !wasPaid {
			paidAt = today
		} else if o.PaidAt != nil {
			paidAt = *o.PaidAt
		} else {
			paidAt = today
		}
		if paidAt.After(today) {
			writeAppError(c, apperr.New(http.StatusUnprocessableEntity, "paid_at cannot be in the future", "VALIDATION_ERROR"))
			return
		}
		o.IsPaid = true
		o.PaidAt = &paidAt
		if !wasPaid {
			o.PaidBy = &user.ID
			if o.Payment == nil {
				p, _ := s.St.GetRecurringPayment(o.RecurringPaymentID)
				o.Payment = p
			}
			_, err = service.EnsureNextOccurrence(s.St, s.Cfg, o.Payment)
			if err != nil {
				c.Status(http.StatusInternalServerError)
				return
			}
		} else if o.PaidBy == nil {
			o.PaidBy = &user.ID
		}
	} else {
		o.IsPaid = false
		o.PaidAt = nil
		o.PaidBy = nil
	}
	if err := s.St.UpdateOccurrence(o); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	updated, _ := s.St.GetOccurrence(id)
	if updated == nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	if updated.IsPaid && !wasPaid && updated.Payment != nil {
		paidAtStr := dates.FormatDate(today)
		if updated.PaidAt != nil {
			paidAtStr = dates.FormatDate(*updated.PaidAt)
		}
		paidBy := user.ID
		if updated.PaidBy != nil {
			paidBy = *updated.PaidBy
		}
		notify.PostWebhookAsync(s.Cfg, notify.EventPaymentOccurrencePaid, notify.PaidOccurrenceData{
			OccurrenceID: updated.ID, RecurringPaymentID: updated.RecurringPaymentID,
			Title: updated.Payment.Title, Amount: updated.Payment.Amount.StringFixed(2),
			Currency: updated.Payment.Currency, DueDate: dates.FormatDate(updated.DueDate),
			PaidAt: paidAtStr, PaidBy: paidBy,
		})
	}
	s.writeOccurrenceJSON(c, updated)
}

func (s *Server) summary(c *gin.Context) {
	from, to := dates.MonthBounds(s.Cfg.Timezone)
	if v := c.Query("from"); v != "" {
		from, _ = dates.ParseDate(v)
	}
	if v := c.Query("to"); v != "" {
		to, _ = dates.ParseDate(v)
	}

	rows, err := s.St.ListOccurrencesForSummary(from, to)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	by := map[string]currencySummary{
		"RUB": {Paid: "0", Unpaid: "0", Overdue: "0", Total: "0"},
		"USD": {Paid: "0", Unpaid: "0", Overdue: "0", Total: "0"},
		"EUR": {Paid: "0", Unpaid: "0", Overdue: "0", Total: "0"},
	}
	for _, o := range rows {
		if o.Payment == nil {
			continue
		}
		cur := o.Payment.Currency
		b := by[cur]
		amt := o.Payment.Amount
		if o.IsPaid {
			b.Paid = decAdd(b.Paid, amt)
		} else if dates.IsOverdue(o.DueDate, o.IsPaid, s.Cfg.Timezone) {
			b.Overdue = decAdd(b.Overdue, amt)
		} else {
			b.Unpaid = decAdd(b.Unpaid, amt)
		}
		b.Total = decAdd(b.Total, amt)
		by[cur] = b
	}
	c.JSON(http.StatusOK, summaryOut{
		Period:     periodRange{From: dates.FormatDate(from), To: dates.FormatDate(to)},
		ByCurrency: by,
	})
}

func decAdd(s string, d decimal.Decimal) string {
	cur, _ := decimal.NewFromString(s)
	return cur.Add(d).StringFixed(2)
}

func (s *Server) dueOccurrences(c *gin.Context) {
	today := dates.TodayInTZ(s.Cfg.Timezone)
	rows, err := s.St.ListDueForReminder(today, s.Cfg.ReminderDaysBefore)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	out := make([]dueOccurrenceOut, 0, len(rows))
	for _, o := range rows {
		if o.Payment == nil {
			continue
		}
		out = append(out, dueOccurrenceOut{
			ID: o.ID, Title: o.Payment.Title, Amount: o.Payment.Amount,
			Currency: o.Payment.Currency, DueDate: dates.FormatDate(o.DueDate),
			DaysLeft: dates.DaysLeft(o.DueDate, s.Cfg.Timezone),
		})
	}
	c.JSON(http.StatusOK, out)
}

func (s *Server) markReminded(c *gin.Context) {
	id, _ := strconvAtoi(c.Param("id"))
	o, err := s.St.GetOccurrence(id)
	if err != nil || o == nil {
		writeAppError(c, apperr.New(http.StatusNotFound, "Occurrence not found", "NOT_FOUND"))
		return
	}
	if o.ReminderSentAt != nil {
		c.Status(http.StatusOK)
		return
	}
	now := time.Now().UTC()
	o.ReminderSentAt = &now
	if err := s.St.UpdateOccurrence(o); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusNoContent)
}

func strconvAtoi(s string) (int, error) {
	var n int
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, apperr.ValidationError
		}
		n = n*10 + int(ch-'0')
	}
	return n, nil
}
