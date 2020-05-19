package auth

import (
	"context"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/syrilster/migrate-leave-krow-to-xero/internal/model"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

type Service struct {
}

func NewAuthService() *Service {
	return &Service{}
}

func (service Service) OAuthService(ctx context.Context, code string) (*model.XeroResponse, error) {
	ctxLogger := log.WithContext(ctx)
	ctxLogger.Infof("Inside the MigrateLeaveKrowToXero service")
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", "http://localhost:8080/v1/oauth/redirect")

	// to get our access token
	reqURL := fmt.Sprint("https://identity.xero.com/connect/token")
	req, err := http.NewRequest(http.MethodPost, reqURL, strings.NewReader(data.Encode()))
	if err != nil {
		ctxLogger.WithError(err).Error("could not create HTTP request")
		return nil, err
	}
	req.Header.Add("Authorization", "Basic MjY1MjFCODdEOTJGNDI5QUI3RkRBQUZGMTk2Rjk5MjI6eHpHR1ZJMkNYc1RPZGFnUTgxVzVBSkdtRWdONG9SeWdVOXFhS2VKOFJaWDkwM2xV")
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("accept", "application/json")

	// Send out the HTTP request
	httpClient := http.Client{}
	res, err := httpClient.Do(req)
	if err != nil {
		ctxLogger.WithError(err).Error("could not send HTTP request")
		return nil, err
	}
	defer res.Body.Close()

	// Parse the request body into the `XeroResponse` struct
	var resp *model.XeroResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		ctxLogger.WithError(err).Error("could not parse JSON response")
		return nil, err
	}

	file, err := json.MarshalIndent(resp, "", " ")
	if err != nil {
		ctxLogger.WithError(err).Error("Error preparing the json to write to file")
		return nil, err
	}

	err = ioutil.WriteFile("xero_session.json", file, 0644)
	if err != nil {
		ctxLogger.WithError(err).Error("Error writing token to file")
		return nil, err
	}
	return resp, nil
}
