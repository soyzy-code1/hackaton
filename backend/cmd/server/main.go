package main

import (
	"context"
	"log"
	"os"

	"invoicekeeper/internal/config"
	"invoicekeeper/internal/db"
	"invoicekeeper/internal/handlers"
	"invoicekeeper/internal/notify"
	"invoicekeeper/internal/service"
	"invoicekeeper/internal/store"
)

func main() {
	cfg := config.Load()
	database, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	if err := db.Migrate(database); err != nil {
		log.Fatal(err)
	}
	st := &store.Store{DB: database}
	if err := service.EnsureAdmin(st, cfg); err != nil {
		log.Fatal(err)
	}
	// Probe the SMTP transport once at boot: a misconfigured host or a blocked
	// egress port is the most common reason reminders never arrive, and the
	// probe makes the cause visible in the container log without sending mail.
	notify.LogSMTPStatus(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service.StartNotifyWorker(ctx, cfg, st)
	router := handlers.NewRouter(cfg, st)
	addr := ":8000"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}
	if err := router.Run(addr); err != nil {
		log.Fatal(err)
	}
}
