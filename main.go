package main

import (
	"github.com/syrilster/migrate-leave-krow-to-xero/internal"
	"github.com/syrilster/migrate-leave-krow-to-xero/internal/config"
)

func main() {
	cfg := config.NewApplicationConfig()
	server := internal.SetupServer(cfg)
	server.Start("", cfg.ServerPort())
}
