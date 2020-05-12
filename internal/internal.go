package internal

import (
	"fmt"
	"github.com/syrilster/migrate-leave-krow-to-xero/internal/config"
	"net/http"

	"github.com/syrilster/migrate-leave-krow-to-xero/internal/middlewares"
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
}

func SetupServer(cfg ServerConfig) *config.Server {
	basePath := fmt.Sprintf("/%v", cfg.Version())
	server := config.NewServer().
		WithRoutes(
			"", StatusRoute(),
		).
		WithRoutes(
			basePath,
		)
	return server
}
