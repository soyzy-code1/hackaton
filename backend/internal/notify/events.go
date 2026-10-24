package notify

import "time"

const (
	EventPaymentDueReminder      = "payment.due_reminder"
	EventRecurringPaymentCreated = "recurring_payment.created"
	EventRecurringPaymentDeleted = "recurring_payment.deleted"
	EventPaymentOccurrencePaid   = "payment_occurrence.paid"
)

type Envelope struct {
	Event     string      `json:"event"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

type DueReminderData struct {
	OccurrenceID       int    `json:"occurrence_id"`
	RecurringPaymentID int    `json:"recurring_payment_id"`
	Title              string `json:"title"`
	Amount             string `json:"amount"`
	Currency           string `json:"currency"`
	DueDate            string `json:"due_date"`
	DaysLeft           int    `json:"days_left"`
}

type RecurringPaymentData struct {
	ID              int    `json:"id"`
	Title           string `json:"title"`
	Amount          string `json:"amount"`
	Currency        string `json:"currency"`
	NextPaymentDate string `json:"next_payment_date"`
}

type PaidOccurrenceData struct {
	OccurrenceID       int    `json:"occurrence_id"`
	RecurringPaymentID int    `json:"recurring_payment_id"`
	Title              string `json:"title"`
	Amount             string `json:"amount"`
	Currency           string `json:"currency"`
	DueDate            string `json:"due_date"`
	PaidAt             string `json:"paid_at"`
	PaidBy             int    `json:"paid_by"`
}
