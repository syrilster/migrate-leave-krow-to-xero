package auth

import (
	log "github.com/sirupsen/logrus"
	"github.com/syrilster/migrate-leave-krow-to-xero/internal/util"
	"net/http"
)

func OauthRedirectHandler(handler OAuthHandler) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		contextLogger := log.WithContext(ctx)
		err := r.ParseForm()
		if err != nil {
			contextLogger.WithError(err).Error("could not parse incoming query")
			util.WithBodyAndStatus(nil, http.StatusBadRequest, w)
		}
		code := r.FormValue("code")

		_, err = handler.OAuthService(ctx, code)
		if err != nil {
			contextLogger.WithError(err).Error("Failed to fetch the access token")
			util.WithBodyAndStatus("Failed to connect to Xero", http.StatusBadRequest, w)
			return
		}

		util.WithBodyAndStatus("Connected to Xero", http.StatusOK, w)
	}
}
