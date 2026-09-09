package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"gryphdash/internal/logging"
)

func main() {
	mode := "web"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}
	if mode == "help" || mode == "-h" || mode == "--help" {
		printUsage()
		return
	}
	closeLogs, logErr := logging.Setup("gryphdash")
	if logErr != nil {
		log.Printf("logging setup failed: %v", logErr)
	} else {
		defer func() { _ = closeLogs() }()
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var err error
	switch mode {
	case "web":
		err = runWeb(ctx)
	case "tui":
		err = runTUI(ctx)
	default:
		printUsage()
		os.Exit(2)
	}
	if err != nil {
		log.Print(err)
	}
}

func printUsage() {
	fmt.Println("Usage: gryphdash [web|tui]")
	fmt.Println("  web  serve the browser dashboard (default)")
	fmt.Println("  tui  display the dashboard in the terminal")
}
