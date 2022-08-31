package errors

import (
	"encoding/json"
	"fmt"
	"github.com/go-playground/validator/v10"
	log "github.com/sirupsen/logrus"
	"net/http"
)

type ErrorHandler struct {
	endpoint string
}

type jsonError struct {
	ErrorMsg string `json:"error"`
}

func NewErrorHandler(endpoint string) *ErrorHandler {
	return &ErrorHandler{endpoint}
}

func (eh *ErrorHandler) WriteAndLogError(
	w http.ResponseWriter,
	msg string,
	err error,
	statusCode int,
	fields log.Fields,
) {
	fields["endpoint"] = eh.endpoint
	logErr := fmt.Errorf("%s: %w", msg, err)
	responseErr := ""
	if statusCode >= 500 {
		log.WithFields(fields).Error(logErr)
		responseErr = msg
	} else {
		log.WithFields(fields).Debug(logErr)
		responseErr = logErr.Error()
	}
	eh.writeErrorMsg(w, responseErr, statusCode)
}

func (eh *ErrorHandler) WriteAndLogErrorMsg(
	w http.ResponseWriter,
	msg string,
	statusCode int,
	fields log.Fields,
) {
	fields["endpoint"] = eh.endpoint
	if statusCode >= 500 {
		log.WithFields(fields).Error(msg)
	} else {
		log.WithFields(fields).Debug(msg)
	}
	eh.writeErrorMsg(w, msg, statusCode)
}

func (eh *ErrorHandler) WriteAndLogValidationErrors(
	w http.ResponseWriter,
	err validator.ValidationErrors,
	statusCode int,
	fields log.Fields,
) {
	eh.writeErrorMsg(w, "validation error", http.StatusBadRequest)
	//TODO
}

func (eh *ErrorHandler) writeErrorMsg(w http.ResponseWriter, msg string, statusCode int) {
	resp, _ := json.Marshal(jsonError{msg})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(resp)
}
