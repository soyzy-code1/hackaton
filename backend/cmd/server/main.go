package main

import (
	"context"
	"log"
	"os"

	"invoicekeeper/internal/config"
	"invoicekeeper/internal/db"
	"invoicekeeper/internal/handlers"
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
