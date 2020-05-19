package handler

import (
	log "github.com/sirupsen/logrus"
	"net/http"
)

func DefaultHandler() func(res http.ResponseWriter, req *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		contextLogger := log.WithContext(ctx)
		contextLogger.Info("Inside Default Handler")
		/*reqURL := fmt.Sprintf("https://login.xero.com/identity/connect/authorize?response_type=%s&client_id=%s&redirect_uri=%s&scope=%s&state=%s",
			"code", "26521B87D92F429AB7FDAAFF196F9922", "http://localhost:8080/oauth/redirect", "offline_access openid payroll.employees", "123")
		req, err := http.NewRequest(http.MethodGet, reqURL, nil)
		if err != nil {
			contextLogger.WithError(err).Error("could not create HTTP request")
			util.WithBodyAndStatus(nil, http.StatusBadRequest, w)
		}
		req.Header.Set("accept", "application/json")

		// Send out the HTTP request
		httpClient := http.Client{}
		res, err := httpClient.Do(req)
		if err != nil {
			contextLogger.WithError(err).Error("could not send HTTP request")
			util.WithBodyAndStatus(nil, http.StatusInternalServerError, w)
		}
		defer res.Body.Close()*/
	}
}
