package internal

import (
	"context"
	"github.com/syrilster/migrate-leave-krow-to-xero/internal/config"
	"github.com/syrilster/migrate-leave-krow-to-xero/internal/model"
	"net/http"
)

type XeroAPIHandler interface {
	MigrateLeaveKrowToXero(ctx context.Context) (model.XeroEmployees, error)
}

func Route(xeroHandler XeroAPIHandler) (route config.Route) {
	route = config.Route{
		Path:    "/krowToXero",
		Method:  http.MethodGet,
		Handler: Handler(xeroHandler),
	}

	return route
}
