package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"go-work/internal/http/constants"
	herrors "go-work/internal/http/errors"
	"go-work/internal/http/validation"
	"go-work/internal/model"
	"mime"
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

type jobServer struct {
	storage  model.JobStorage
	validate *validator.Validate
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	js, err := json.Marshal(v)
	if err != nil {
		http.Error(w, "error forming response data", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

var (
	createJobErrorHandler    = herrors.NewErrorHandler("CreateJob")
	getJobErrorHandler       = herrors.NewErrorHandler("GetJob")
	getJobByNameErrorHandler = herrors.NewErrorHandler("GetJobByName")
	deleteJobErrorHandler    = herrors.NewErrorHandler("DeleteJob")
)

type requestJob struct {
	Name          string   `json:"name" validate:"required,uniqueName"`
	CrontabString string   `json:"crontabString" validate:"required,crontabString"`
	Command       string   `json:"command" validate:"required"`
	Arguments     []string `json:"arguments"`
	Timeout       uint     `json:"timeout" validate:"required"`
}

type responseId struct {
	Id model.JobId `json:"id"`
}

func (js *jobServer) createJobHandler(w http.ResponseWriter, req *http.Request) {

	contentType := req.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		createJobErrorHandler.WriteAndLogError(
			w,
			"failed to parse media type",
			err, http.StatusBadRequest,
			log.Fields{"header": contentType},
		)
		return
	}
	if mediaType != "application/json" {
		createJobErrorHandler.WriteAndLogError(
			w,
			"expect application/json Content-Type",
			errors.New("Content-Type error"),
			http.StatusUnsupportedMediaType,
			log.Fields{"media type": mediaType},
		)
		return
	}
	dec := json.NewDecoder(req.Body)
	dec.DisallowUnknownFields()
	rj := requestJob{}
	if err = dec.Decode(&rj); err != nil {
		createJobErrorHandler.WriteAndLogError(
			w,
			"failed to parse request body",
			err,
			http.StatusBadRequest,
			log.Fields{},
		)
		return
	}

	background := context.Background()
	err = js.validate.StructCtx(background, rj)
	if err != nil {
		createJobErrorHandler.WriteAndLogValidationErrors(
			w,
			err.(validator.ValidationErrors),
			log.Fields{"request job": rj},
		)
		return
	}

	timeoutCtx, cancel := context.WithTimeout(background, constants.StorageOperationTimeout)
	defer cancel()
	id, err := js.storage.CreateJob(
		timeoutCtx,
		rj.Name,
		rj.CrontabString,
		rj.Command,
		rj.Arguments,
		rj.Timeout,
	)
	if err != nil {
		createJobErrorHandler.WriteAndLogError(
			w,
			"failed to save new job",
			err,
			http.StatusInternalServerError,
			log.Fields{"request job": rj},
		)
		return
	}
	writeJSON(w, responseId{id})
}

func (js *jobServer) getJobHandler(w http.ResponseWriter, req *http.Request) {
	id, _ := strconv.ParseInt(mux.Vars(req)["id"], 10, 64)
	timeoutCtx, cancel := context.WithTimeout(context.Background(), constants.StorageOperationTimeout)
	defer cancel()
	job, err := js.storage.GetJob(timeoutCtx, model.JobId(id))
	if err != nil {
		statusCode := http.StatusNotFound
		if !errors.Is(err, model.ErrorNotFound) {
			statusCode = http.StatusInternalServerError
		}
		getJobErrorHandler.WriteAndLogError(
			w,
			fmt.Sprintf("failed to get job by id %d", id),
			err,
			statusCode,
			log.Fields{},
		)
		return
	}
	writeJSON(w, job)
}

func (js *jobServer) getJobByNameHandler(w http.ResponseWriter, req *http.Request) {
	name := mux.Vars(req)["name"]
	timeoutCtx, cancel := context.WithTimeout(context.Background(), constants.StorageOperationTimeout)
	defer cancel()
	job, err := js.storage.GetJobByName(timeoutCtx, name)
	if err != nil {
		statusCode := http.StatusNotFound
		if !errors.Is(err, model.ErrorNotFound) {
			statusCode = http.StatusInternalServerError
		}
		getJobByNameErrorHandler.WriteAndLogError(
			w,
			fmt.Sprintf("failed to get job by name %s", name),
			err,
			statusCode,
			log.Fields{},
		)
		return
	}
	writeJSON(w, job)
}

func (js *jobServer) deleteJobHandler(w http.ResponseWriter, req *http.Request) {
	id, _ := strconv.ParseInt(mux.Vars(req)["id"], 10, 64)
	timeoutCtx, cancel := context.WithTimeout(context.Background(), constants.StorageOperationTimeout)
	defer cancel()
	err := js.storage.DeleteJob(timeoutCtx, model.JobId(id))
	if err != nil {
		deleteJobErrorHandler.WriteAndLogError(
			w,
			fmt.Sprintf("failed to delete job with id %d", id),
			err,
			http.StatusInternalServerError,
			log.Fields{},
		)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Infof("%s %s", r.Method, r.RequestURI)
		next.ServeHTTP(w, r)
	})
}

func NewJobServer(storage model.JobStorage, addr string) (*http.Server, error) {
	server := jobServer{storage, validator.New()}
	err := validation.RegisterJobValidation(server.validate, storage)
	server.validate.RegisterTagNameFunc(func(field reflect.StructField) string {
		fullJson := field.Tag.Get("json")
		if fullJson == "-" {
			return ""
		}
		jsonName := strings.SplitN(fullJson, ",", 2)[0]
		if jsonName != "" {
			return jsonName
		}
		return field.Name
	})
	if err != nil {
		return nil, fmt.Errorf("error registering job validation: %w", err)
	}

	router := mux.NewRouter()
	router.StrictSlash(true)
	router.HandleFunc("/api/v1/job/", server.createJobHandler).Methods("POST")
	router.HandleFunc("/api/v1/job/{id:[0-9]+}/", server.getJobHandler).Methods("GET")
	router.HandleFunc("/api/v1/job/{id:[0-9]+}/", server.deleteJobHandler).Methods("DELETE")
	router.HandleFunc("/api/v1/job/{name:[a-zA-Z_]\\w*}/", server.getJobByNameHandler).Methods("GET")
	router.Use(loggingMiddleware)
	router.StrictSlash(true)
	return &http.Server{Addr: addr, Handler: router}, nil
}
