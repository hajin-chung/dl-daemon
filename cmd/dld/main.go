package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"deps.me/dl-daemon/internal/db"
	"deps.me/dl-daemon/internal/manager"
)

func main() {
	database, err := db.OpenDatabase()
	if err != nil {
		log.Fatalf("open database: %v", err)
	}

	mgr := manager.New(database)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := mgr.Run(ctx); err != nil {
		log.Fatalf("run manager: %v", err)
	}
}

