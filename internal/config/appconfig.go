package config

import (
	"github.com/syrilster/migrate-leave-krow-to-xero/internal/xero"
	"net/http"
	_ "os"
	"time"

	"github.com/syrilster/migrate-leave-krow-to-xero/internal/customhttp"
)

type ApplicationConfig struct {
	envValues  *envConfig
	xeroClient xero.ClientInterface
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

//XeroEndpoint returns the xero endpoint
func (cfg *ApplicationConfig) XeroEndpoint() xero.ClientInterface {
	return cfg.xeroClient
}

//XeroKey returns the xero client id
func (cfg *ApplicationConfig) XeroKey() string {
	return cfg.envValues.XeroKey
}

//XeroSecret returns the xero client secret
func (cfg *ApplicationConfig) XeroSecret() string {
	return cfg.envValues.XeroSecret
}

//XeroAuthEndpoint returns the auth related endpoint
func (cfg *ApplicationConfig) XeroAuthEndpoint() string {
	return cfg.envValues.XeroAuthEndpoint
}

//XeroRedirectURI returns the redirect URI
func (cfg *ApplicationConfig) XeroRedirectURI() string {
	return cfg.envValues.XeroRedirectURI
}

//XlsFileLocation returns the file location to read the leave requests
func (cfg *ApplicationConfig) XlsFileLocation() string {
	return cfg.envValues.XlsFileLocation
}

//NewApplicationConfig loads config values from environment and initialises config
func NewApplicationConfig() *ApplicationConfig {
	envValues := NewEnvironmentConfig()
	httpCommand := NewHTTPCommand()
	xeroClient := xero.NewClient(envValues.XeroEndpoint, httpCommand)
	return &ApplicationConfig{
		envValues:  envValues,
		xeroClient: xeroClient,
	}
}

// NewHTTPCommand returns the HTTP client
func NewHTTPCommand() customhttp.HTTPCommand {
	httpCommand := customhttp.New(
		customhttp.WithHTTPClient(&http.Client{Timeout: 5 * time.Second}),
	).Build()

	return httpCommand
}
