package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"invoicekeeper/internal/config"
	"invoicekeeper/internal/dates"
	"invoicekeeper/internal/models"
	"invoicekeeper/internal/notify"
	"invoicekeeper/internal/store"
)

func StartNotifyWorker(ctx context.Context, cfg config.Config, st *store.Store) {
	if !cfg.SMTPConfigured() && !cfg.WebhookConfigured() {
		return
	}
	interval := time.Duration(cfg.NotifyPollSec) * time.Second
	if interval < 10*time.Second {
		interval = 10 * time.Second
	}
	go func() {
		log.Printf("notify worker: started (poll %s)", interval)
		runNotifyCycle(cfg, st)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runNotifyCycle(cfg, st)
			}
		}
	}()
}

func runNotifyCycle(cfg config.Config, st *store.Store) {
	today := dates.TodayInTZ(cfg.Timezone)
	horizon := cfg.ReminderDaysBefore
	if !cfg.SMTPConfigured() && !cfg.WebhookConfigured() {
		log.Printf("notify worker: no channels configured (SMTP_ENABLED=%t, SMTP_HOST=%q, SMTP_FROM=%q, SMTP_TO=%q)", cfg.SMTPEnabled, cfg.SMTPHost, cfg.SMTPFrom, cfg.SMTPTo)
	}

	if cfg.SMTPEnabled && !cfg.SMTPConfigured() {
		log.Printf("notify: SMTP is enabled but incomplete; set SMTP_HOST, SMTP_PORT, SMTP_USER, SMTP_PASSWORD, SMTP_FROM and SMTP_TO")
	}
	if cfg.SMTPConfigured() {
		items, err := st.ListDueForEmailReminder(today, horizon)
		if err != nil {
			log.Printf("notify: email list: %v", err)
		} else {
			log.Printf("notify: found %d items for email", len(items))
			for _, o := range items {
				if o.Payment == nil {
					continue
				}
				data := notify.DueReminderData{
					OccurrenceID:       o.ID,
					RecurringPaymentID: o.RecurringPaymentID,
					Title:              o.Payment.Title,
					Amount:             o.Payment.Amount.StringFixed(2),
					Currency:           o.Payment.Currency,
					DueDate:            dates.FormatDate(o.DueDate),
					DaysLeft:           dates.DaysLeft(o.DueDate, cfg.Timezone),
				}
				if err := notify.SendDueReminderEmail(cfg, data); err != nil {
					log.Printf("notify: email send error for occurrence %d: %v", o.ID, err)
					continue
				}
				createAssignedNotifications(st, o, "reminder", "Email: "+data.Title, fmt.Sprintf("%s — %s %s, срок %s", data.Title, data.Amount, data.Currency, data.DueDate), "email")
				if err := st.MarkEmailReminderSent(o.ID); err != nil {
					log.Printf("notify: mark email sent %d: %v", o.ID, err)
				}
			}
		}
	}

	if cfg.WebhookConfigured() {
		items, err := st.ListDueForWebhookReminder(today, horizon)
		if err != nil {
			log.Printf("notify: webhook due list: %v", err)
		} else {
			for _, o := range items {
				if o.Payment == nil {
					continue
				}
				
				data := notify.DueReminderData{
					OccurrenceID:       o.ID,
					RecurringPaymentID: o.RecurringPaymentID,
					Title:              o.Payment.Title,
					Amount:             o.Payment.Amount.StringFixed(2),
					Currency:           o.Payment.Currency,
					DueDate:            dates.FormatDate(o.DueDate),
					DaysLeft:           dates.DaysLeft(o.DueDate, cfg.Timezone),
				}
				if notify.PostWebhook(cfg, notify.EventPaymentDueReminder, data) {
					if err := st.MarkWebhookReminderSent(o.ID); err != nil {
						log.Printf("notify: mark webhook sent %d: %v", o.ID, err)
					}
				}
			}
		}
	}
}

func createAssignedNotifications(st *store.Store, occurrence models.PaymentOccurrence, notificationType, title, message, channel string) {
	assigned, err := st.PaymentAssignees(occurrence.RecurringPaymentID)
	if err != nil {
		log.Printf("notify: load assignees for occurrence %d: %v", occurrence.ID, err)
		return
	}
	for _, userID := range assigned {
		target := userID
		if err := st.CreateNotification(&models.Notification{
			Type: notificationType, Title: title, Message: message,
			OccurrenceID: &occurrence.ID, UserID: &target, Channel: channel,
		}); err != nil {
			log.Printf("notify: create notification for user %d: %v", userID, err)
		}
	}
}
