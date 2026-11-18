package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"rfc-server/internal/config"
	"rfc-server/internal/server"
)

func main() {
	path := flag.String("config", "mock.yml", "YAML configuration")
	validate := flag.Bool("check", false, "validate configuration and exit")
	flag.Parse()
	cfg, err := config.Load(*path)
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}

	if *validate {
		log.Print("configuration valid")
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := server.Run(ctx, cfg); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
