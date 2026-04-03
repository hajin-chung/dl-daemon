package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"deps.me/dl-daemon/internal/db"
	"deps.me/dl-daemon/internal/logging"
	"deps.me/dl-daemon/internal/manager"
	"deps.me/dl-daemon/internal/model"
	"deps.me/dl-daemon/internal/platform/chzzk"
	"deps.me/dl-daemon/internal/web"
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
	case "config":
		handleConfig(database, args[1:])
	case "chzzk":
		handleChzzk(database, args[1:])
	case "download", "downloads":
		handleDownloads(database, args[1:])
	case "web":
		handleWeb(database, args[1:])
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
			fmt.Fprintln(os.Stderr, "usage: dld target add <platform> <id> [label] [output_dir]")
			os.Exit(1)
		}
		label := ""
		if len(args) >= 4 {
			label = args[3]
		}
		outputDir := ""
		if len(args) >= 5 {
			outputDir = args[4]
		}
		target := model.Target{Platform: args[1], Id: args[2], Label: label, OutputDir: outputDir}
		if err := database.AddTarget(target); err != nil {
			log.Fatalf("add target: %v", err)
		}
		slog.Info("target added", "platform", target.Platform, "id", target.Id, "label", target.Label, "output_dir", target.OutputDir)
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
			fmt.Printf("%s\t%s", target.Platform, target.Id)
			if target.Label != "" {
				fmt.Printf("\t%s", target.Label)
			}
			if target.OutputDir != "" {
				fmt.Printf("\t%s", target.OutputDir)
			}
			fmt.Println()
		}
	case "set-output":
		if len(args) != 4 {
			fmt.Fprintln(os.Stderr, "usage: dld target set-output <platform> <id> <output_dir>")
			os.Exit(1)
		}
		if err := database.SetTargetOutputDir(args[1], args[2], args[3]); err != nil {
			log.Fatalf("set target output: %v", err)
		}
		slog.Info("target output updated", "platform", args[1], "id", args[2], "output_dir", args[3])
		fmt.Printf("Updated target output: %s %s -> %s\n", args[1], args[2], args[3])
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

func handleConfig(database *db.DB, args []string) {
	if len(args) == 0 {
		printConfigHelp()
		os.Exit(1)
	}

	switch args[0] {
	case "set":
		if len(args) != 3 {
			fmt.Fprintln(os.Stderr, "usage: dld config set <key> <value>")
			os.Exit(1)
		}
		if err := database.SetMetadata(args[1], args[2]); err != nil {
			log.Fatalf("set config: %v", err)
		}
		slog.Info("config updated", "key", args[1])
		fmt.Printf("Set %s\n", args[1])
	case "get":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "usage: dld config get <key>")
			os.Exit(1)
		}
		value, err := database.GetMetadata(args[1])
		if err != nil {
			if err == sql.ErrNoRows {
				fmt.Fprintf(os.Stderr, "config key not found: %s\n", args[1])
				os.Exit(1)
			}
			log.Fatalf("get config: %v", err)
		}
		fmt.Println(value)
	case "list", "ls":
		rows, err := database.ListMetadata()
		if err != nil {
			log.Fatalf("list config: %v", err)
		}
		if len(rows) == 0 {
			fmt.Println("No config values stored.")
			return
		}
		for _, row := range rows {
			fmt.Printf("%s\t%s\n", row.Key, maskConfigValue(row.Key, row.Value))
		}
	default:
		printConfigHelp()
		os.Exit(1)
	}
}

func handleChzzk(database *db.DB, args []string) {
	if len(args) == 0 {
		printChzzkHelp()
		os.Exit(1)
	}

	switch args[0] {
	case "me":
		aut, err := getConfigFallback(database, "chzzk.nid_aut", "NID_AUT")
		if err != nil {
			if err == sql.ErrNoRows {
				fmt.Fprintln(os.Stderr, "missing config: chzzk.nid_aut (or legacy NID_AUT)")
				os.Exit(1)
			}
			log.Fatalf("get chzzk auth: %v", err)
		}
		ses, err := getConfigFallback(database, "chzzk.nid_ses", "NID_SES")
		if err != nil {
			if err == sql.ErrNoRows {
				fmt.Fprintln(os.Stderr, "missing config: chzzk.nid_ses (or legacy NID_SES)")
				os.Exit(1)
			}
			log.Fatalf("get chzzk session: %v", err)
		}

		client := chzzk.NewClient(aut, ses)
		status, err := client.GetUserStatus()
		if err != nil {
			log.Fatalf("chzzk me: %v", err)
		}

		if status.Content.NickName != nil && *status.Content.NickName != "" {
			fmt.Printf("Authorized as %s\n", *status.Content.NickName)
			return
		}
		fmt.Println("Token appears invalid or user status is unavailable.")
	default:
		printChzzkHelp()
		os.Exit(1)
	}
}

func handleWeb(database *db.DB, args []string) {
	addr := "127.0.0.1:8080"
	if len(args) >= 1 && strings.TrimSpace(args[0]) != "" {
		addr = args[0]
	}

	server, err := web.New(database)
	if err != nil {
		log.Fatalf("init web server: %v", err)
	}

	slog.Info("web ui starting", "addr", addr)
	fmt.Printf("dld web listening on http://%s\n", addr)
	if err := http.ListenAndServe(addr, server.Handler()); err != nil {
		log.Fatalf("serve web ui: %v", err)
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
		fmt.Printf("%s\t%s\t%s\t%d/%d\t%s\n", download.Platform, download.VideoID, download.Status, download.BytesWritten, download.TotalBytes, download.Title)
		if download.ErrorMsg != nil && *download.ErrorMsg != "" {
			fmt.Printf("  error: %s\n", *download.ErrorMsg)
		}
	}
}

func getConfigFallback(database *db.DB, keys ...string) (string, error) {
	for _, key := range keys {
		value, err := database.GetMetadata(key)
		if err == nil {
			return value, nil
		}
		if err != sql.ErrNoRows {
			return "", err
		}
	}
	return "", sql.ErrNoRows
}

func maskConfigValue(key string, value string) string {
	lower := strings.ToLower(key)
	if strings.Contains(lower, "token") || strings.Contains(lower, "aut") || strings.Contains(lower, "ses") || strings.Contains(lower, "secret") || strings.Contains(lower, "password") {
		if len(value) <= 8 {
			return "********"
		}
		return value[:4] + strings.Repeat("*", len(value)-8) + value[len(value)-4:]
	}
	return value
}

func printHelp() {
	fmt.Println("dld")
	fmt.Println("Usage:")
	fmt.Println("  dld run")
	fmt.Println("  dld target <subcommand>")
	fmt.Println("  dld config <subcommand>")
	fmt.Println("  dld chzzk <subcommand>")
	fmt.Println("  dld downloads")
	fmt.Println("  dld web [addr]")
	fmt.Println()
	fmt.Println("Target commands:")
	fmt.Println("  dld target add <platform> <id> [label] [output_dir]")
	fmt.Println("  dld target list")
	fmt.Println("  dld target set-output <platform> <id> <output_dir>")
	fmt.Println("  dld target remove <platform> <id>")
	fmt.Println()
	fmt.Println("Config commands:")
	fmt.Println("  dld config set <key> <value>")
	fmt.Println("  dld config get <key>")
	fmt.Println("  dld config list")
	fmt.Println()
	fmt.Println("Chzzk commands:")
	fmt.Println("  dld chzzk me")
}

func printTargetHelp() {
	fmt.Println("dld target")
	fmt.Println("Usage:")
	fmt.Println("  dld target add <platform> <id> [label] [output_dir]")
	fmt.Println("  dld target list")
	fmt.Println("  dld target set-output <platform> <id> <output_dir>")
	fmt.Println("  dld target remove <platform> <id>")
}

func printConfigHelp() {
	fmt.Println("dld config")
	fmt.Println("Usage:")
	fmt.Println("  dld config set <key> <value>")
	fmt.Println("  dld config get <key>")
	fmt.Println("  dld config list")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  dld config set chzzk.nid_aut <value>")
	fmt.Println("  dld config set chzzk.nid_ses <value>")
}

func printChzzkHelp() {
	fmt.Println("dld chzzk")
	fmt.Println("Usage:")
	fmt.Println("  dld chzzk me")
	fmt.Println()
	fmt.Println("Description:")
	fmt.Println("  Check whether the configured Chzzk auth tokens are valid.")
}
