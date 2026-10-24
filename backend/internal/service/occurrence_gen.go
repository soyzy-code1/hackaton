package service

import (
	"invoicekeeper/internal/config"
	"invoicekeeper/internal/dates"
	"invoicekeeper/internal/models"
	"invoicekeeper/internal/store"
)

func EnsureNextOccurrence(st *store.Store, cfg config.Config, payment *models.RecurringPayment) (*models.PaymentOccurrence, error) {
	if !payment.IsActive {
		return nil, nil
	}
	last, err := st.LastOccurrence(payment.ID)
	if err != nil {
		return nil, err
	}
	if last == nil {
		occ := &models.PaymentOccurrence{
			RecurringPaymentID: payment.ID,
			DueDate:            payment.NextPaymentDate,
		}
		if err := st.CreateOccurrence(occ); err != nil {
			return nil, err
		}
		return occ, nil
	}
	if !last.IsPaid {
		return nil, nil
	}
	nextDue := dates.AddMonthClamped(last.DueDate, cfg.Timezone)
	payment.NextPaymentDate = nextDue
	if err := st.UpdateRecurringPayment(payment); err != nil {
		return nil, err
	}
	occ := &models.PaymentOccurrence{
		RecurringPaymentID: payment.ID,
		DueDate:            nextDue,
	}
	if err := st.CreateOccurrence(occ); err != nil {
		return nil, err
	}
	return occ, nil
}

func LazyEnsureActive(st *store.Store, cfg config.Config) error {
	payments, err := st.ListActivePaymentsForLazy()
	if err != nil {
		return err
	}
	for i := range payments {
		_, err := EnsureNextOccurrence(st, cfg, &payments[i])
		if err != nil {
			return err
		}
	}
	return nil
}
