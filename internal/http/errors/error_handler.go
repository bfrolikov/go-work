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
	message string,
	err error,
	statusCode int,
	fields log.Fields,
) {
	fields["endpoint"] = eh.endpoint
	logErr := fmt.Sprintf("%s: %s", message, err)
	if statusCode >= 500 {
		log.WithFields(fields).Error(logErr)
	} else {
		log.WithFields(fields).Debug(logErr)
	}
	eh.writeJsonErrorMsg(w, message, statusCode)
}

func (eh *ErrorHandler) WriteAndLogValidationErrors(
	w http.ResponseWriter,
	err validator.ValidationErrors,
	fields log.Fields,
) {
	fields["endpoint"] = eh.endpoint
	fieldErrors := make(map[string]string)
	for _, fieldError := range err {
		fieldErrors[fieldError.Field()] = fieldError.Error()
	}
	log.WithFields(fields).Debugf("Received json with invalid fields: %s", fieldErrors)
	fields["field errors"] = fieldErrors
	resp, _ := json.Marshal(fieldErrors)
	eh.writeJson(w, resp, http.StatusUnprocessableEntity)
}

func (eh *ErrorHandler) writeJson(w http.ResponseWriter, jsonMsg []byte, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(jsonMsg)
}

func (eh *ErrorHandler) writeJsonErrorMsg(w http.ResponseWriter, msg string, statusCode int) {
	resp, _ := json.Marshal(jsonError{msg})
	eh.writeJson(w, resp, statusCode)
}
