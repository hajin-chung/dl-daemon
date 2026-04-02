package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"deps.me/dl-daemon/internal/db"
	"deps.me/dl-daemon/internal/logging"
	"deps.me/dl-daemon/internal/manager"
	"deps.me/dl-daemon/internal/model"
)

func main() {
	logPath, err := logging.Setup(os.Stderr)
	if err != nil {
		log.Fatalf("setup logging: %v", err)
	}

	database, err := db.OpenDatabase()
	if err != nil {
		log.Fatalf("open database: %v", err)
	}

	slog.Info("logging initialized", "log_path", logPath)
	slog.Info("database opened")

	args := os.Args[1:]
	if len(args) == 0 {
		runDaemon(database)
		return
	}

	switch args[0] {
	case "run":
		runDaemon(database)
	case "target":
		handleTarget(database, args[1:])
	case "download", "downloads":
		handleDownloads(database, args[1:])
	case "help", "-h", "--help":
		printHelp()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", args[0])
		printHelp()
		os.Exit(1)
	}
}

func runDaemon(database *db.DB) {
	slog.Info("daemon starting")
	mgr := manager.New(database)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := mgr.Run(ctx); err != nil {
		slog.Error("manager stopped with error", "error", err)
		log.Fatalf("run manager: %v", err)
	}
	slog.Info("daemon stopped")
}

func handleTarget(database *db.DB, args []string) {
	if len(args) == 0 {
		printTargetHelp()
		os.Exit(1)
	}

	switch args[0] {
	case "add":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: dld target add <platform> <id> [label]")
			os.Exit(1)
		}
		label := ""
		if len(args) >= 4 {
			label = args[3]
		}
		target := model.Target{Platform: args[1], Id: args[2], Label: label}
		if err := database.AddTarget(target); err != nil {
			log.Fatalf("add target: %v", err)
		}
		slog.Info("target added", "platform", target.Platform, "id", target.Id, "label", target.Label)
		fmt.Printf("Added target: %s %s", target.Platform, target.Id)
		if target.Label != "" {
			fmt.Printf(" (%s)", target.Label)
		}
		fmt.Println()
	case "list", "ls":
		targets, err := database.GetTargets()
		if err != nil {
			log.Fatalf("get targets: %v", err)
		}
		if len(targets) == 0 {
			fmt.Println("No targets configured.")
			return
		}
		for _, target := range targets {
			if target.Label != "" {
				fmt.Printf("%s\t%s\t%s\n", target.Platform, target.Id, target.Label)
			} else {
				fmt.Printf("%s\t%s\n", target.Platform, target.Id)
			}
		}
	case "remove", "rm":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: dld target remove <platform> <id>")
			os.Exit(1)
		}
		if err := database.RemoveTarget(args[1], args[2]); err != nil {
			log.Fatalf("remove target: %v", err)
		}
		slog.Info("target removed", "platform", args[1], "id", args[2])
		fmt.Printf("Removed target: %s %s\n", args[1], args[2])
	default:
		printTargetHelp()
		os.Exit(1)
	}
}

func handleDownloads(database *db.DB, args []string) {
	downloads, err := database.ListDownloads()
	if err != nil {
		log.Fatalf("list downloads: %v", err)
	}
	if len(downloads) == 0 {
		fmt.Println("No downloads recorded.")
		return
	}

	_ = args
	for _, download := range downloads {
		if download.ErrorMsg != "" {
			fmt.Printf("%s\t%s\t%s\t%d/%d\t%s\n", download.Platform, download.VideoID, download.Status, download.BytesWritten, download.TotalBytes, download.Title)
			fmt.Printf("  error: %s\n", download.ErrorMsg)
		} else {
			fmt.Printf("%s\t%s\t%s\t%d/%d\t%s\n", download.Platform, download.VideoID, download.Status, download.BytesWritten, download.TotalBytes, download.Title)
		}
	}
}

func printHelp() {
	fmt.Println("dld")
	fmt.Println("Usage:")
	fmt.Println("  dld run")
	fmt.Println("  dld target add <platform> <id> [label]")
	fmt.Println("  dld target list")
	fmt.Println("  dld target remove <platform> <id>")
	fmt.Println("  dld downloads")
}

func printTargetHelp() {
	fmt.Println("dld target")
	fmt.Println("Usage:")
	fmt.Println("  dld target add <platform> <id> [label]")
	fmt.Println("  dld target list")
	fmt.Println("  dld target remove <platform> <id>")
}
