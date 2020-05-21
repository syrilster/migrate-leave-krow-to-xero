package internal

import (
	"fmt"
	"github.com/syrilster/migrate-leave-krow-to-xero/internal/auth"
	"github.com/syrilster/migrate-leave-krow-to-xero/internal/config"
	"github.com/syrilster/migrate-leave-krow-to-xero/internal/middlewares"
	"github.com/syrilster/migrate-leave-krow-to-xero/internal/xero"
	"net/http"
)

//StatusRoute health check route
func StatusRoute() (route config.Route) {
	route = config.Route{
		Path:    "/status",
		Method:  http.MethodGet,
		Handler: middlewares.RuntimeHealthCheck(),
	}
	return route
}

type ServerConfig interface {
	Version() string
	BaseURL() string
	XeroEndpoint() xero.ClientInterface
	XeroKey() string
	XeroSecret() string
	XeroAuthEndpoint() string
}

func SetupServer(cfg ServerConfig) *config.Server {
	basePath := fmt.Sprintf("/%v", cfg.Version())
	service := NewService(cfg.XeroEndpoint())
	authService := auth.NewAuthService(cfg.XeroKey(), cfg.XeroSecret(), cfg.XeroAuthEndpoint())
	server := config.NewServer().
		WithRoutes(
			"", StatusRoute(),
		).
		WithRoutes(
			basePath,
			Route(service),
			auth.Route(authService),
		)
	return server
}
