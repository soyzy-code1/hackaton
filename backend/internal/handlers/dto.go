package handlers

import (
	"time"

	"github.com/shopspring/decimal"

	"invoicekeeper/internal/dates"
	"invoicekeeper/internal/models"
)

type userOut struct {
	ID        int       `json:"id"`
	Login     string    `json:"login"`
	Email     *string   `json:"email"`
	FullName  *string   `json:"full_name"`
	Role      string    `json:"role"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

func toUserOut(u *models.User) userOut {
	return userOut{
		ID: u.ID, Login: u.Login, Email: u.Email, FullName: u.FullName,
		Role: u.Role, IsActive: u.IsActive, CreatedAt: u.CreatedAt,
	}
}

type recurringPaymentOut struct {
	ID              int             `json:"id"`
	Title           string          `json:"title"`
	Amount          decimal.Decimal `json:"amount"`
	Currency        string          `json:"currency"`
	Periodicity     string          `json:"periodicity"`
	Category        string          `json:"category"`
	NextPaymentDate string          `json:"next_payment_date"`
	IsActive        bool            `json:"is_active"`
	AssignedUserIDs []int           `json:"assigned_user_ids"`
	CreatedAt       time.Time       `json:"created_at"`
}

func toPaymentOut(p *models.RecurringPayment) recurringPaymentOut {
	return recurringPaymentOut{
		ID: p.ID, Title: p.Title, Amount: p.Amount, Currency: p.Currency,
		Periodicity: p.Periodicity, Category: p.Category, NextPaymentDate: dates.FormatDate(p.NextPaymentDate),
		IsActive: p.IsActive, AssignedUserIDs: p.AssignedUserIDs, CreatedAt: p.CreatedAt,
	}
}

type occurrenceOut struct {
	ID                 int              `json:"id"`
	RecurringPaymentID int              `json:"recurring_payment_id"`
	DueDate            string           `json:"due_date"`
	IsPaid             bool             `json:"is_paid"`
	PaidAt              *string          `json:"paid_at"`
	ActualPaymentDate   *string          `json:"actual_payment_date"`
	PaidBy              *int             `json:"paid_by"`
	CreatedAt          time.Time        `json:"created_at"`
	Title              *string          `json:"title"`
	Amount             *decimal.Decimal `json:"amount"`
	Currency           *string          `json:"currency"`
	IsOverdue          bool             `json:"is_overdue"`
	DaysLeft           *int             `json:"days_left"`
}

func toOccurrenceOut(o *models.PaymentOccurrence, tz string) occurrenceOut {
	var paidAt *string
	if o.PaidAt != nil {
		s := dates.FormatDate(*o.PaidAt)
		paidAt = &s
	}
	var title *string
	var amount *decimal.Decimal
	var currency *string
	if o.Payment != nil {
		title = &o.Payment.Title
		amount = &o.Payment.Amount
		currency = &o.Payment.Currency
	}
	dl := dates.DaysLeft(o.DueDate, tz)
	return occurrenceOut{
		ID: o.ID, RecurringPaymentID: o.RecurringPaymentID,
		DueDate: dates.FormatDate(o.DueDate), IsPaid: o.IsPaid,
		PaidAt: paidAt, ActualPaymentDate: paidAt, PaidBy: o.PaidBy, CreatedAt: o.CreatedAt,
		Title: title, Amount: amount, Currency: currency,
		IsOverdue: dates.IsOverdue(o.DueDate, o.IsPaid, tz),
		DaysLeft:  &dl,
	}
}

// Контракт фронта: только обязательные поля (GET ?view=minimal, PATCH ?view=minimal).
type occurrenceMinimalOut struct {
	ID                  int     `json:"id"`
	Title               string  `json:"title"`
	Amount              string  `json:"amount"`
	Currency            string  `json:"currency"`
	DueDate             string  `json:"due_date"`
	IsPaid              bool    `json:"is_paid"`
	ActualPaymentDate   *string `json:"actual_payment_date"`
}

func toOccurrenceMinimal(o *models.PaymentOccurrence, _ string) occurrenceMinimalOut {
	title, amount, currency := "", "0.00", ""
	if o.Payment != nil {
		title = o.Payment.Title
		amount = o.Payment.Amount.StringFixed(2)
		currency = o.Payment.Currency
	}
	var actual *string
	if o.PaidAt != nil {
		s := dates.FormatDate(*o.PaidAt)
		actual = &s
	}
	return occurrenceMinimalOut{
		ID: o.ID, Title: title, Amount: amount, Currency: currency,
		DueDate: dates.FormatDate(o.DueDate), IsPaid: o.IsPaid,
		ActualPaymentDate: actual,
	}
}

type dueOccurrenceOut struct {
	ID       int             `json:"id"`
	Title    string          `json:"title"`
	Amount   decimal.Decimal `json:"amount"`
	Currency string          `json:"currency"`
	DueDate  string          `json:"due_date"`
	DaysLeft int             `json:"days_left"`
}

type currencySummary struct {
	Paid    string `json:"paid"`
	Unpaid  string `json:"unpaid"`
	Overdue string `json:"overdue"`
	Total   string `json:"total"`
}

type periodRange struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type summaryOut struct {
	Period     periodRange                `json:"period"`
	ByCurrency map[string]currencySummary `json:"by_currency"`
}
