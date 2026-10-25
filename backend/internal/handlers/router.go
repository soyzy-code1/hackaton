package handlers

import (
	"net/http"
	"strings"
	"sync"
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
	// Serialize occurrence transitions so duplicate callbacks cannot both
	// generate the next occurrence from the same unpaid row.
	occurrenceMu sync.Mutex
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
	payments.PATCH("/:id", requireAdmin, s.updateRecurringPayment)
	payments.DELETE("/:id", requireAdmin, s.deleteRecurringPayment)

	occ := v1.Group("/payment-occurrences", authMiddleware(cfg, st))
	occ.GET("", s.listOccurrences)
	occ.GET("/:id", s.getOccurrence)
	occ.PATCH("/:id", s.patchOccurrence)

	v1.GET("/payments/summary", authMiddleware(cfg, st), s.summary)
	v1.POST("/chat/gigachat", authMiddleware(cfg, st), requireUser, s.chatWithGigaChat)
	v1.POST("/assistant/subscription-analysis", authMiddleware(cfg, st), s.analyzeSubscriptions)
	v1.GET("/audit-log", authMiddleware(cfg, st), requireAdmin, s.listAuditLogs)
	v1.GET("/categories", authMiddleware(cfg, st), s.listCategories)
	v1.POST("/categories", authMiddleware(cfg, st), requireAdmin, s.createCategory)
	v1.DELETE("/categories/:name", authMiddleware(cfg, st), requireAdmin, s.deleteCategory)
	v1.GET("/analytics/categories", authMiddleware(cfg, st), s.categoryAnalytics)

	budgets := v1.Group("/budgets", authMiddleware(cfg, st))
	budgets.GET("", s.listBudgets)
	budgets.PUT("/:currency", s.updateBudget)

	internal := v1.Group("/internal", internalTokenMiddleware(cfg))
	internal.GET("/due-occurrences", s.dueOccurrences)
	internal.POST("/occurrences/:id/mark-reminded", s.markReminded)
	internal.POST("/notifications", s.createNotification)

	notifications := v1.Group("/notifications", authMiddleware(cfg, st))
	notifications.GET("", s.listNotifications)
	notifications.PATCH("/:id", s.markNotificationRead)
	notifications.POST("/read-all", s.markAllNotificationsRead)

	return r
}

type notificationOut struct {
	ID           int       `json:"id"`
	Type         string    `json:"type"`
	Title        string    `json:"title"`
	Message      string    `json:"message"`
	OccurrenceID *int      `json:"occurrence_id"`
	IsRead       bool      `json:"is_read"`
	CreatedAt    time.Time `json:"created_at"`
	Channels     []string  `json:"channels"`
}

func toNotificationOut(n models.Notification) notificationOut {
	return notificationOut{ID: n.ID, Type: n.Type, Title: n.Title, Message: n.Message, OccurrenceID: n.OccurrenceID, IsRead: n.IsRead, CreatedAt: n.CreatedAt, Channels: []string{n.Channel}}
}

func (s *Server) createNotification(c *gin.Context) {
	var body struct {
		Type         string `json:"type"`
		Title        string `json:"title"`
		Message      string `json:"message"`
		OccurrenceID *int   `json:"occurrence_id"`
		UserID       *int   `json:"user_id"`
		Channel      string `json:"channel"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Title == "" || body.Channel == "" {
		writeAppError(c, apperr.ValidationError)
		return
	}
	if body.Type == "" {
		body.Type = "reminder"
	}
	// Channel workers send only the occurrence id. Fan out to every assigned user.
	if body.UserID == nil && body.OccurrenceID != nil {
		occurrence, err := s.St.GetOccurrence(*body.OccurrenceID)
		if err != nil || occurrence == nil {
			writeAppError(c, apperr.New(http.StatusNotFound, "Occurrence not found", "NOT_FOUND"))
			return
		}
		assigned, err := s.St.PaymentAssignees(occurrence.RecurringPaymentID)
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		for _, id := range assigned {
			target := id
			if err := s.St.CreateNotification(&models.Notification{Type: body.Type, Title: body.Title, Message: body.Message, OccurrenceID: body.OccurrenceID, UserID: &target, Channel: body.Channel}); err != nil {
				c.Status(http.StatusInternalServerError)
				return
			}
		}
		c.Status(http.StatusCreated)
		return
	}
	if err := s.St.CreateNotification(&models.Notification{Type: body.Type, Title: body.Title, Message: body.Message, OccurrenceID: body.OccurrenceID, UserID: body.UserID, Channel: body.Channel}); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusCreated)
}

func (s *Server) listNotifications(c *gin.Context) {
	page, _ := strconvAtoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconvAtoi(c.DefaultQuery("page_size", "20"))
	user, _ := currentUser(c)
	var userID *int
	if user.Role != "admin" {
		userID = &user.ID
	}
	items, total, err := s.St.ListNotifications(page, pageSize, c.Query("unread_only") == "true", userID)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	out := make([]notificationOut, 0, len(items))
	for _, item := range items {
		out = append(out, toNotificationOut(item))
	}
	c.JSON(http.StatusOK, gin.H{"items": out, "total": total, "page": page, "page_size": pageSize})
}

func (s *Server) markNotificationRead(c *gin.Context) {
	id, _ := strconvAtoi(c.Param("id"))
	user, _ := currentUser(c)
	var userID *int
	if user.Role != "admin" {
		userID = &user.ID
	}
	if err := s.St.MarkNotificationRead(id, userID); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) markAllNotificationsRead(c *gin.Context) {
	user, _ := currentUser(c)
	var userID *int
	if user.Role != "admin" {
		userID = &user.ID
	}
	if err := s.St.MarkAllNotificationsRead(userID); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusNoContent)
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
	// Browser sessions are authenticated only through this HttpOnly cookie.
	// The JWT is intentionally not returned in the response body.
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "access_token",
		Value:    token,
		Path:     "/",
		MaxAge:   24 * 60 * 60,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   c.Request.TLS != nil,
	})
	_ = s.St.CreateAuditLog(&models.AuditLog{UserID: &user.ID, Action: "login", Entity: "user", EntityID: &user.ID, Details: "successful authentication"})
	c.JSON(http.StatusOK, gin.H{"user": toUserOut(user)})
}

func (s *Server) listCategories(c *gin.Context) {
	rows, err := s.St.DB.Query(`SELECT name FROM payment_categories WHERE is_active = 1 ORDER BY name`)
	if err != nil { c.Status(http.StatusInternalServerError); return }
	defer rows.Close()
	out := []string{}
	for rows.Next() { var name string; if err := rows.Scan(&name); err != nil { c.Status(http.StatusInternalServerError); return }; out = append(out, name) }
	c.JSON(http.StatusOK, out)
}

func (s *Server) createCategory(c *gin.Context) {
	var body struct { Name string `json:"name"` }
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Name) == "" { writeAppError(c, apperr.ValidationError); return }
	name := strings.TrimSpace(body.Name)
	if _, err := s.St.DB.Exec(`INSERT INTO payment_categories (name) VALUES (?)`, name); err != nil { writeAppError(c, apperr.New(http.StatusConflict, "Category already exists", "CATEGORY_EXISTS")); return }
	c.JSON(http.StatusCreated, gin.H{"name": name})
}

func (s *Server) deleteCategory(c *gin.Context) {
	name := strings.TrimSpace(c.Param("name"))
	if name == "" || name == "other" { writeAppError(c, apperr.ValidationError); return }
	_, err := s.St.DB.Exec(`UPDATE payment_categories SET is_active = 0 WHERE name = ?`, name)
	if err != nil { c.Status(http.StatusInternalServerError); return }
	c.Status(http.StatusNoContent)
}

func (s *Server) categoryAnalytics(c *gin.Context) {
	currency := strings.ToUpper(strings.TrimSpace(c.DefaultQuery("currency", "RUB")))
	isActive := true
	payments, err := s.St.ListRecurringPaymentsByCategory(nil, &currency, &isActive)
	if err != nil { c.Status(http.StatusInternalServerError); return }
	type item struct { Category string `json:"category"`; Amount decimal.Decimal `json:"amount"`; Count int `json:"count"` }
	by := map[string]*item{}
	for _, p := range payments { category := firstNonEmpty(p.Category, "other"); if by[category] == nil { by[category] = &item{Category: category} }; by[category].Amount = by[category].Amount.Add(p.Amount); by[category].Count++ }
	out := make([]item, 0, len(by)); for _, value := range by { out = append(out, *value) }
	c.JSON(http.StatusOK, gin.H{"currency": currency, "items": out})
}

func (s *Server) listAuditLogs(c *gin.Context) {
	page, _ := strconvAtoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconvAtoi(c.DefaultQuery("page_size", "25"))
	if page < 1 { page = 1 }
	if pageSize < 1 || pageSize > 100 { pageSize = 25 }
	items, total, err := s.St.ListAuditLogs(page, pageSize, strings.TrimSpace(c.Query("action")))
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	out := make([]gin.H, 0, len(items))
	for _, item := range items {
		out = append(out, gin.H{
			"id": item.ID, "user_id": item.UserID, "action": item.Action,
			"entity": item.Entity, "entity_id": item.EntityID, "details": item.Details,
			"created_at": item.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": out, "total": total, "page": page, "page_size": pageSize})
}

func (s *Server) me(c *gin.Context) {
	u, _ := currentUser(c)
	c.JSON(http.StatusOK, toUserOut(u))
}

func (s *Server) logout(c *gin.Context) {
	token := s.authTokenFromRequest(c)
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
		Name: "access_token", Value: "", Path: "/", MaxAge: -1,
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
	user, _ := currentUser(c)
	if user.Role != "admin" {
		filtered := items[:0]
		for _, item := range items {
			for _, assignedID := range item.AssignedUserIDs {
				if assignedID == user.ID {
					filtered = append(filtered, item)
					break
				}
			}
		}
		items = filtered
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
		Category        string          `json:"category"`
		AssignedUserIDs []int           `json:"assigned_user_ids"`
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
		Periodicity: body.Periodicity, Category: firstNonEmpty(body.Category, "other"), NextPaymentDate: nd, IsActive: true, CreatedBy: admin.ID,
	}
	if err := s.St.CreateRecurringPayment(p); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	assigned := body.AssignedUserIDs
	if len(assigned) == 0 {
		users, err := s.St.ListUsers()
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		for _, candidate := range users {
			if candidate.Role == "user" && candidate.IsActive {
				assigned = append(assigned, candidate.ID)
			}
		}
	}
	if err := s.St.SetPaymentAssignees(p.ID, assigned); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	p.AssignedUserIDs = assigned
	occurrence, err := service.EnsureNextOccurrence(s.St, s.Cfg, p)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	if occurrence != nil && s.Cfg.SMTPConfigured() {
		data := notify.DueReminderData{
			OccurrenceID: occurrence.ID, RecurringPaymentID: p.ID,
			Title: p.Title, Amount: p.Amount.StringFixed(2), Currency: p.Currency,
			DueDate:  dates.FormatDate(occurrence.DueDate),
			DaysLeft: dates.DaysLeft(occurrence.DueDate, s.Cfg.Timezone),
		}
		if err := notify.SendPaymentCreatedEmail(s.Cfg, data); err != nil {
			c.Error(err)
		} else {
			occurrenceID := occurrence.ID
			_ = s.St.CreateNotification(&models.Notification{
				Type: "created", Title: "Email: создан платеж " + p.Title,
				Message:      p.Title + " — " + p.Amount.StringFixed(2) + " " + p.Currency + ", срок " + data.DueDate,
				OccurrenceID: &occurrenceID, UserID: &admin.ID, Channel: "email",
			})
		}
	}
	notify.PostWebhookAsync(s.Cfg, notify.EventRecurringPaymentCreated, notify.RecurringPaymentData{
		ID: p.ID, Title: p.Title, Amount: p.Amount.StringFixed(2),
		Currency: p.Currency, NextPaymentDate: dates.FormatDate(p.NextPaymentDate),
	})
	adminID := admin.ID
	_ = s.St.CreateAuditLog(&models.AuditLog{UserID: &adminID, Action: "create", Entity: "recurring_payment", EntityID: &p.ID, Details: p.Title})
	c.JSON(http.StatusCreated, toPaymentOut(p))
}

func (s *Server) updateRecurringPayment(c *gin.Context) {
	id, err := strconvAtoi(c.Param("id"))
	if err != nil {
		writeAppError(c, apperr.ValidationError)
		return
	}
	payment, err := s.St.GetRecurringPayment(id)
	if err != nil || payment == nil || payment.DeletedAt != nil {
		writeAppError(c, apperr.New(http.StatusNotFound, "Payment not found", "NOT_FOUND"))
		return
	}
	var body struct {
		Title           *string          `json:"title"`
		Amount          *decimal.Decimal `json:"amount"`
		Currency        *string          `json:"currency"`
		NextPaymentDate *string          `json:"next_payment_date"`
		Periodicity     *string          `json:"periodicity"`
		Category        *string          `json:"category"`
		IsActive        *bool            `json:"is_active"`
		AssignedUserIDs *[]int           `json:"assigned_user_ids"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeAppError(c, apperr.ValidationError)
		return
	}
	if body.Title != nil {
		payment.Title = strings.TrimSpace(*body.Title)
		if payment.Title == "" {
			writeAppError(c, apperr.ValidationError)
			return
		}
	}
	if body.Amount != nil {
		if body.Amount.LessThanOrEqual(decimal.Zero) {
			writeAppError(c, apperr.ValidationError)
			return
		}
		payment.Amount = *body.Amount
	}
	if body.Currency != nil {
		payment.Currency = strings.ToUpper(strings.TrimSpace(*body.Currency))
	}
	if body.NextPaymentDate != nil {
		date, parseErr := dates.ParseDate(*body.NextPaymentDate)
		if parseErr != nil {
			writeAppError(c, apperr.ValidationError)
			return
		}
		payment.NextPaymentDate = date
	}
	if body.Periodicity != nil {
		payment.Periodicity = strings.TrimSpace(*body.Periodicity)
		if payment.Periodicity == "" {
			writeAppError(c, apperr.ValidationError)
			return
		}
	}
	if body.Category != nil {
		payment.Category = firstNonEmpty(*body.Category, "other")
	}
	if body.IsActive != nil {
		payment.IsActive = *body.IsActive
	}
	if body.AssignedUserIDs != nil {
		assigned, validationErr := s.validatePaymentAssignees(*body.AssignedUserIDs)
		if validationErr != nil {
			writeAppError(c, validationErr)
			return
		}
		if err := s.St.SetPaymentAssignees(id, assigned); err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		payment.AssignedUserIDs = assigned
	}
	if err := s.St.UpdateRecurringPayment(payment); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	if body.AssignedUserIDs == nil {
		payment.AssignedUserIDs, _ = s.St.PaymentAssignees(id)
	}
	admin, _ := currentUser(c)
	adminID := admin.ID
	_ = s.St.CreateAuditLog(&models.AuditLog{UserID: &adminID, Action: "update", Entity: "recurring_payment", EntityID: &payment.ID, Details: payment.Title})
	c.JSON(http.StatusOK, toPaymentOut(payment))
}

func (s *Server) validatePaymentAssignees(ids []int) ([]int, *apperr.AppError) {
	users, err := s.St.ListUsers()
	if err != nil {
		return nil, apperr.New(http.StatusInternalServerError, "Unable to load users", "INTERNAL_ERROR")
	}
	valid := make(map[int]bool, len(users))
	for _, user := range users {
		if user.Role == "user" && user.IsActive {
			valid[user.ID] = true
		}
	}
	seen := make(map[int]bool, len(ids))
	out := make([]int, 0, len(ids))
	for _, userID := range ids {
		if !valid[userID] {
			return nil, apperr.New(http.StatusUnprocessableEntity, "Invalid payment assignee", "VALIDATION_ERROR")
		}
		if !seen[userID] {
			seen[userID] = true
			out = append(out, userID)
		}
	}
	return out, nil
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
	admin, _ := currentUser(c)
	adminID := admin.ID
	_ = s.St.CreateAuditLog(&models.AuditLog{UserID: &adminID, Action: "delete", Entity: "recurring_payment", EntityID: &id, Details: p.Title})
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
	user, _ := currentUser(c)
	var userID *int
	if user.Role != "admin" {
		userID = &user.ID
	}
	rows, err := s.St.ListOccurrences(currency, rid, from, to, userID)
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
	if err != nil || o == nil || !s.userCanAccessOccurrence(c, o) {
		writeAppError(c, apperr.New(http.StatusNotFound, "Occurrence not found", "NOT_FOUND"))
		return
	}
	s.writeOccurrenceJSON(c, o)
}

func (s *Server) userCanAccessOccurrence(c *gin.Context, occurrence *models.PaymentOccurrence) bool {
	user, _ := currentUser(c)
	if user == nil || user.Role == "admin" {
		return true
	}
	assigned, err := s.St.PaymentAssignees(occurrence.RecurringPaymentID)
	if err != nil {
		return false
	}
	for _, id := range assigned {
		if id == user.ID {
			return true
		}
	}
	return false
}

func (s *Server) writeOccurrenceJSON(c *gin.Context, o *models.PaymentOccurrence) {
	if c.Query("view") == "minimal" {
		c.JSON(http.StatusOK, toOccurrenceMinimal(o, s.Cfg.Timezone))
		return
	}
	c.JSON(http.StatusOK, toOccurrenceOut(o, s.Cfg.Timezone))
}

func (s *Server) patchOccurrence(c *gin.Context) {
	s.occurrenceMu.Lock()
	defer s.occurrenceMu.Unlock()

	id, _ := strconvAtoi(c.Param("id"))
	o, err := s.St.GetOccurrence(id)
	if err != nil || o == nil || !s.userCanAccessOccurrence(c, o) {
		writeAppError(c, apperr.New(http.StatusNotFound, "Occurrence not found", "NOT_FOUND"))
		return
	}
	var body struct {
		IsPaid            bool    `json:"is_paid"`
		PaidAt            *string `json:"paid_at"`
		ActualPaymentDate *string `json:"actual_payment_date"`
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
	details := "payment status changed"
	if updated.Payment != nil {
		details = updated.Payment.Title
	}
	_ = s.St.CreateAuditLog(&models.AuditLog{UserID: &user.ID, Action: map[bool]string{true: "mark_paid", false: "mark_unpaid"}[updated.IsPaid], Entity: "payment_occurrence", EntityID: &updated.ID, Details: details})
	s.writeOccurrenceJSON(c, updated)
}

func (s *Server) listBudgets(c *gin.Context) {
	user, _ := currentUser(c)
	budgets, err := s.St.GetBudgets(user.ID)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	monthFrom, monthTo := dates.MonthBounds(s.Cfg.Timezone)
	today := dates.TodayInTZ(s.Cfg.Timezone)
	yearFrom := time.Date(today.Year(), time.January, 1, 0, 0, 0, 0, time.UTC)
	yearTo := time.Date(today.Year(), time.December, 31, 0, 0, 0, 0, time.UTC)
	out := make([]gin.H, 0, 3)
	for _, currency := range []string{"RUB", "USD", "EUR"} {
		limits := budgets[currency]
		monthlyLimit, _ := decimal.NewFromString(limits[0])
		yearlyLimit, _ := decimal.NewFromString(limits[1])
		spentMonth, err := s.St.SumPaidOccurrences(user.ID, currency, monthFrom, monthTo)
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		spentYear, err := s.St.SumPaidOccurrences(user.ID, currency, yearFrom, yearTo)
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		out = append(out, gin.H{
			"currency":        currency,
			"monthly_limit":   monthlyLimit.StringFixed(2),
			"yearly_limit":    yearlyLimit.StringFixed(2),
			"spent_month":     spentMonth.StringFixed(2),
			"spent_year":      spentYear.StringFixed(2),
			"remaining_month": decimal.Max(decimal.Zero, monthlyLimit.Sub(spentMonth)).StringFixed(2),
			"remaining_year":  decimal.Max(decimal.Zero, yearlyLimit.Sub(spentYear)).StringFixed(2),
		})
	}
	c.JSON(http.StatusOK, out)
}

func (s *Server) updateBudget(c *gin.Context) {
	currency := strings.ToUpper(strings.TrimSpace(c.Param("currency")))
	if currency != "RUB" && currency != "USD" && currency != "EUR" {
		writeAppError(c, apperr.ValidationError)
		return
	}
	var body struct {
		MonthlyLimit *decimal.Decimal `json:"monthly_limit"`
		YearlyLimit  *decimal.Decimal `json:"yearly_limit"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.MonthlyLimit == nil || body.YearlyLimit == nil || body.MonthlyLimit.IsNegative() || body.YearlyLimit.IsNegative() {
		writeAppError(c, apperr.ValidationError)
		return
	}
	user, _ := currentUser(c)
	if err := s.St.UpsertBudget(user.ID, currency, body.MonthlyLimit.StringFixed(2), body.YearlyLimit.StringFixed(2)); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	monthFrom, monthTo := dates.MonthBounds(s.Cfg.Timezone)
	today := dates.TodayInTZ(s.Cfg.Timezone)
	yearFrom := time.Date(today.Year(), time.January, 1, 0, 0, 0, 0, time.UTC)
	yearTo := time.Date(today.Year(), time.December, 31, 0, 0, 0, 0, time.UTC)
	spentMonth, err := s.St.SumPaidOccurrences(user.ID, currency, monthFrom, monthTo)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	spentYear, err := s.St.SumPaidOccurrences(user.ID, currency, yearFrom, yearTo)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"currency":        currency,
		"monthly_limit":   body.MonthlyLimit.StringFixed(2),
		"yearly_limit":    body.YearlyLimit.StringFixed(2),
		"spent_month":     spentMonth.StringFixed(2),
		"spent_year":      spentYear.StringFixed(2),
		"remaining_month": decimal.Max(decimal.Zero, body.MonthlyLimit.Sub(spentMonth)).StringFixed(2),
		"remaining_year":  decimal.Max(decimal.Zero, body.YearlyLimit.Sub(spentYear)).StringFixed(2),
	})
}

func (s *Server) summary(c *gin.Context) {
	from, to := dates.MonthBounds(s.Cfg.Timezone)
	if v := c.Query("from"); v != "" {
		from, _ = dates.ParseDate(v)
	}
	if v := c.Query("to"); v != "" {
		to, _ = dates.ParseDate(v)
	}

	user, _ := currentUser(c)
	var userID *int
	if user.Role != "admin" {
		userID = &user.ID
	}
	rows, err := s.St.ListOccurrencesForSummary(from, to, userID)
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
