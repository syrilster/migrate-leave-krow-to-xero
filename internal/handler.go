package internal

import (
	"bytes"
	log "github.com/sirupsen/logrus"
	"github.com/syrilster/migrate-leave-krow-to-xero/internal/util"
	"github.com/tealeg/xlsx"
	"io"
	"net/http"
)

const XlsOutputPath = "/Users/syril/sample.xlsx"

func Handler(xeroHandler XeroAPIHandler) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		contextLogger := log.WithContext(ctx)

		err := parseRequestBody(req)
		if err != nil {
			util.WithBodyAndStatus(nil, http.StatusInternalServerError, res)
			return
		}
		resp, err := xeroHandler.MigrateLeaveKrowToXero(ctx)
		if err != nil {
			contextLogger.WithError(err).Error("Failed to fetch details from Xero")
			util.WithBodyAndStatus(nil, http.StatusInternalServerError, res)
			return
		}
		util.WithBodyAndStatus(resp, http.StatusOK, res)
	}
}

func parseRequestBody(req *http.Request) error {
	ctx := req.Context()
	contextLogger := log.WithContext(ctx)
	err := req.ParseMultipartForm(32 << 20)
	if err != nil {
		contextLogger.WithError(err).Error("Failed to parse request body")
		return err
	}

	file, _, err := req.FormFile("file")
	if err != nil {
		contextLogger.WithError(err).Error("Failed to get the file from request")
		return err
	}
	defer file.Close()

	buf := bytes.NewBuffer(nil)
	if _, err := io.Copy(buf, file); err != nil {
		contextLogger.WithError(err).Error("Failed to copy file contents to buffer")
		return err
	}

	excelFile, err := xlsx.OpenBinary(buf.Bytes())
	if err != nil {
		contextLogger.WithError(err).Error("Failed to convert bytes to excel file")
		return err
	}
	err = excelFile.Save(XlsOutputPath)
	if err != nil {
		contextLogger.WithError(err).Error("Failed to save excel file to disk")
		return err
	}
	return nil
}
