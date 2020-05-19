package internal

import (
	log "github.com/sirupsen/logrus"
	"github.com/syrilster/migrate-leave-krow-to-xero/internal/util"
	"net/http"
)

func Handler(xeroHandler XeroAPIHandler) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		contextLogger := log.WithContext(ctx)
		contextLogger.Info("Inside Handler")

		resp, err := xeroHandler.MigrateLeaveKrowToXero(ctx)
		if err != nil {
			contextLogger.WithError(err).Error("Failed to fetch details from Xero")
			util.WithBodyAndStatus(nil, http.StatusInternalServerError, res)
			return
		}
		util.WithBodyAndStatus(resp, http.StatusOK, res)
	}
}
