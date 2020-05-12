package config

import (
	"net/http"
	"time"

	"github.com/syrilster/migrate-leave-krow-to-xero/internal/customhttp"
)

type ApplicationConfig struct {
	envValues *envConfig
}

//Version returns application version
func (cfg *ApplicationConfig) Version() string {
	return cfg.envValues.Version
}

//ServerPort returns the port no to listen for requests
func (cfg *ApplicationConfig) ServerPort() int {
	return cfg.envValues.ServerPort
}

//BaseURL returns the base URL
func (cfg *ApplicationConfig) BaseURL() string {
	return cfg.envValues.BaseUrl
}

//NewApplicationConfig loads config values from environment and initialises config
func NewApplicationConfig() *ApplicationConfig {
	envValues := newEnvironmentConfig()
	//httpCommand := NewHTTPCommand()

	return &ApplicationConfig{
		envValues: envValues,
	}
}

// NewHTTPCommand returns the HTTP client
func NewHTTPCommand() customhttp.HTTPCommand {
	httpCommand := customhttp.New(
		customhttp.WithHTTPClient(&http.Client{Timeout: 5 * time.Second}),
	).Build()

	return httpCommand
}
