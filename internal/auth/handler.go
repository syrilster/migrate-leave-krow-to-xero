package auth

import (
	log "github.com/sirupsen/logrus"
	"github.com/syrilster/migrate-leave-krow-to-xero/internal/util"
	"net/http"
)

const successRedirectURL = "http://localhost:8080/upload"
const errorRedirectURL = "http://localhost:8080/error"

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
			http.Redirect(w, r, errorRedirectURL, http.StatusSeeOther)
			contextLogger.WithError(err).Error("Failed to fetch the access token")
			util.WithBodyAndStatus("Failed to connect to Xero", http.StatusBadRequest, w)
			return
		}

		http.Redirect(w, r, successRedirectURL, http.StatusSeeOther)
		return
	}
}
