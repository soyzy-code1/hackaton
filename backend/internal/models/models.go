package models

import (
	"time"

	"github.com/shopspring/decimal"
)

type User struct {
	ID           int
	Login        string
	Email        *string
	FullName     *string
	PasswordHash string
	Role         string
	IsActive     bool
	CreatedAt    time.Time
	UpdatedAt    *time.Time
}

type RecurringPayment struct {
	ID              int
	Title           string
	Amount          decimal.Decimal
	Currency        string
	Periodicity     string
	Category        string
	NextPaymentDate time.Time
	IsActive        bool
	CreatedBy       int
	AssignedUserIDs []int
	CreatedAt       time.Time
	UpdatedAt       *time.Time
	DeletedAt       *time.Time
}

type AuditLog struct {
	ID        int
	UserID    *int
	Action    string
	Entity    string
	EntityID  *int
	Details   string
	CreatedAt time.Time
}

type Notification struct {
	ID           int
	Type         string
	Title        string
	Message      string
	OccurrenceID *int
	UserID       *int
	Channel      string
	IsRead       bool
	CreatedAt    time.Time
}

type PaymentOccurrence struct {
	ID                 int
	RecurringPaymentID int
	DueDate            time.Time
	IsPaid             bool
	PaidAt             *time.Time
	PaidBy             *int
	ReminderSentAt        *time.Time
	EmailReminderSentAt   *time.Time
	WebhookReminderSentAt *time.Time
	CreatedAt             time.Time
	UpdatedAt          *time.Time
	Payment            *RecurringPayment
}
