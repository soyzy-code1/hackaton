package service

import (
	"invoicekeeper/internal/auth"
	"invoicekeeper/internal/config"
	"invoicekeeper/internal/models"
	"invoicekeeper/internal/store"
)

func EnsureAdmin(st *store.Store, cfg config.Config) error {
	existing, err := st.GetUserByLogin(cfg.AdminLogin)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}
	hash, err := auth.HashPassword(cfg.AdminPassword)
	if err != nil {
		return err
	}
	email := cfg.AdminEmail
	u := &models.User{
		Login:        cfg.AdminLogin,
		Email:        &email,
		PasswordHash: hash,
		Role:         "admin",
		IsActive:     true,
	}
	return st.CreateUser(u)
}
