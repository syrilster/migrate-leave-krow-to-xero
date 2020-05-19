package config

import (
	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/syrilster/migrate-leave-krow-to-xero/internal/xero"
	"math"
	"net/http"
	"os"
	_ "os"
	"time"

	"github.com/XeroAPI/xerogolang"
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

func (cfg *ApplicationConfig) XeroEndpoint() xero.ClientInterface {
	return cfg.xeroClient
}

func (cfg *ApplicationConfig) XeroProvider() *xerogolang.Provider {
	envValues := NewEnvironmentConfig()
	provider := xerogolang.New(envValues.XeroKey, envValues.XeroSecret, "https://digio.com.au/")
	goth.UseProviders(provider)
	store := sessions.NewFilesystemStore(os.TempDir(), []byte("xero_gothic_session"))
	store.MaxLength(math.MaxInt64)
	gothic.Store = store
	return provider
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
