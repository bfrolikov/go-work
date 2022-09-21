package go_work

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	log "github.com/sirupsen/logrus"
	"go-work/internal/http"
	"go-work/internal/model"
	"go-work/internal/scheduler"
	"go-work/test/data"
	"go-work/test/url"
	nhttp "net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"
)

type testApp struct {
	server   *nhttp.Server
	client   *nhttp.Client
	database *sql.DB
}

const timeout = time.Second * 15

var deleteAllJobsQuery = "DELETE from jobs"

func TestGoWork(t *testing.T) {
	resolveArguments()
	background := context.Background()
	dataSourceName := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("TEST_DB_HOST"),
		os.Getenv("TEST_DB_PORT"),
		"go-work",
		os.Getenv("TEST_DB_PASSWORD"),
		"go-work",
	)
	database, err := sql.Open("postgres", dataSourceName)
	if err != nil {
		t.Fatal(fmt.Errorf("error opening database: %w", err))
	}
	storage, err := model.NewSQLJobStorage(background, "postgres", dataSourceName)
	if err != nil {
		t.Fatal(fmt.Errorf("could not create job storage: %w", err))
	}
	server, err := http.NewJobServer(storage, fmt.Sprintf(":%s", os.Getenv("TEST_SERVER_PORT")))
	if err != nil {
		t.Fatal(fmt.Errorf("could not create job server: %w", err))
	}
	registerServer(background, t, server)

	app := testApp{server, nhttp.DefaultClient, database}
	t.Run("Test REST API", func(t *testing.T) {
		t.Run("Test getting job by id", func(t *testing.T) {
			app.setupApp(background, t)

			existingJob := data.InitialJobs[0]
			job, err := app.getJobById(background, existingJob.Id)
			if err != nil {
				t.Fatal(fmt.Errorf("error getting created job by id %d: %w", existingJob.Id, err))
			}
			requireEqual("name", existingJob.Name, job.Name, t)
			requireEqual("crontabString", existingJob.CrontabString, job.CrontabString, t)
			requireEqual("command", existingJob.Command, job.Command, t)
			requireEqual("timeout", existingJob.Timeout, job.Timeout, t)
		})

		t.Run("Test creating and getting new job by name", func(t *testing.T) {
			app.setupApp(background, t)

			existingJob := data.InitialJobs[0]
			job, err := app.getJobByName(background, existingJob.Name)
			if err != nil {
				t.Fatal(fmt.Errorf("error getting created job by name %s: %w", existingJob.Name, err))
			}
			requireEqual("name", existingJob.Name, job.Name, t)
			requireEqual("crontabString", existingJob.CrontabString, job.CrontabString, t)
			requireEqual("command", existingJob.Command, job.Command, t)
			requireEqual("timeout", existingJob.Timeout, job.Timeout, t)
		})

		t.Run("Test deleting job", func(t *testing.T) {
			app.setupApp(background, t)

			existingJob := data.InitialJobs[0]
			err := app.deleteJob(background, existingJob.Id)
			if err != nil {
				t.Fatal(fmt.Errorf("error deleting job with id %d: %w", existingJob.Id, err))
			}
		})

		t.Run("Test creating already existing job", func(t *testing.T) {
			app.setupApp(background, t)

			existingJob := data.InitialJobs[0]
			existingJobData := data.JobRequestData{
				Name:          existingJob.Name,
				CrontabString: existingJob.CrontabString,
				Command:       existingJob.Command,
				Arguments:     existingJob.Arguments,
				Timeout:       existingJob.Timeout,
			}
			_, err := app.createJob(background, &existingJobData)
			expectErrorStatusCode(err, nhttp.StatusUnprocessableEntity, t)
		})

		t.Run("Test getting nonexistent job", func(t *testing.T) {
			app.setupApp(background, t)

			var maxId model.JobId = 0
			for i, job := range data.InitialJobs {
				if i == 0 || maxId < job.Id {
					maxId = job.Id
				}
			}
			_, err := app.getJobById(background, maxId+1)
			expectErrorStatusCode(err, nhttp.StatusNotFound, t)
		})
	})

	executionData := data.NewExecutionData()

	registerServer(background, t, getPingServer(executionData))
	//log.SetLevel(log.ErrorLevel)
	t.Run("Test job execution", func(t *testing.T) {
		t.Run("Test execution of initial jobs", func(t *testing.T) {
			app.setupApp(background, t)
			cancelCtx, cancel := context.WithCancel(background)
			defer cancel()
			schd := scheduler.New(storage, 1)
			go func() {
				schd.Start(cancelCtx)
			}()
			time.Sleep(2*time.Duration(executionData.MaxInterval())*time.Minute + data.IntervalEps)
			if err := executionData.ValidateExecutionData(); err != nil {
				t.Fatal(fmt.Errorf("error validating execution of tasks: %w", err))
			}
		})
		t.Run("Test execution of initial jobs while modifying database", func(t *testing.T) {
			app.setupApp(background, t)
			cancelCtx, cancel := context.WithCancel(background)
			defer cancel()
			schd := scheduler.New(storage, 1)
			go func() {
				schd.Start(cancelCtx)
			}()
			go func() {
				app.alterDatabase(cancelCtx, t)
			}()
			time.Sleep(2*time.Duration(executionData.MaxInterval())*time.Minute + data.IntervalEps)
			if err := executionData.ValidateExecutionData(); err != nil {
				t.Fatal(fmt.Errorf("error validating execution of tasks: %w", err))
			}
		})
	})
}

func (ta *testApp) alterDatabase(ctx context.Context, t *testing.T) {
	createdJobData := data.JobRequestData{
		Name:          "new_job",
		CrontabString: "* * * 3 *",
		Command:       "pass",
		Timeout:       123000,
	}
	var id model.JobId
	create := true
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
			var err error = nil
			if create {
				id, err = ta.createJob(ctx, &createdJobData)
			} else {
				err = ta.deleteJob(ctx, id)
			}
			if err != nil {
				t.Error(err)
				return
			}
			create = !create
		}
	}
}

func getPingServer(executionData *data.ExecutionData) *nhttp.Server {
	pingRouter := mux.NewRouter()
	pingRouter.StrictSlash(true)
	pingRouter.HandleFunc("/tests/ping/{interval:[0-9]+}/", func(w nhttp.ResponseWriter, req *nhttp.Request) {
		interval, _ := strconv.Atoi(mux.Vars(req)["interval"])
		executionData.SignalExecution(uint(interval))
		log.Infof("Executed job with interval %dmin", interval)
	}).Methods("GET")
	return &nhttp.Server{Addr: fmt.Sprintf(":%s", os.Getenv("TEST_PING_SERVER_PORT")), Handler: pingRouter}
}

func registerServer(ctx context.Context, t *testing.T, server *nhttp.Server) {
	go func() {
		if err := server.ListenAndServe(); err != nil {
			if !errors.Is(err, nhttp.ErrServerClosed) {
				t.Error(fmt.Errorf("error starting server: %w", err))
			}
		}
	}()
	t.Cleanup(func() {
		timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		if err := server.Shutdown(timeoutCtx); err != nil {
			t.Fatal(fmt.Errorf("error while shutting down sever: %w", err))
		}
	})
}

func expectErrorStatusCode(err error, errorStatusCode int, t *testing.T) {
	if err != nil {
		var statusCodeErr *statusCodeError
		if errors.As(err, &statusCodeErr) {
			if statusCodeErr.receivedStatusCode != errorStatusCode {
				t.Fatalf(
					"expected status code %d, received %d",
					nhttp.StatusUnprocessableEntity,
					statusCodeErr.receivedStatusCode,
				)
			}
		} else {
			t.Fatal(err)
		}
	} else {
		t.Fatalf("expected error, got %d", nhttp.StatusOK)
	}
}

func requireEqual[K comparable](name string, first K, second K, t *testing.T) {
	if first != second {
		t.Fatalf("expected %s to be equal, instead got %v and %v", name, first, second)
	}
}

func (ta *testApp) clearDatabase(ctx context.Context, t *testing.T) {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	_, err := ta.database.ExecContext(timeoutCtx, deleteAllJobsQuery)
	if err != nil {
		t.Fatal(fmt.Errorf("error clearing database %w", err))
	}
}

type responseId struct {
	Id model.JobId `json:"id"`
}

type statusCodeError struct {
	expectedStatusCode int
	receivedStatusCode int
	responseBody       string
	responseDecodeErr  error
}

func (e *statusCodeError) Error() string {
	responseString := ""
	if e.responseDecodeErr != nil {
		responseString = fmt.Sprintf(" and an error while decoding response: %s", e.responseDecodeErr.Error())
	} else {
		responseString = fmt.Sprintf(", response body: %s", e.responseBody)
	}
	return fmt.Sprintf(
		"expected status code %d, got %d%s",
		e.expectedStatusCode,
		e.receivedStatusCode,
		responseString,
	)
}

func checkStatusCode(response *nhttp.Response, statusCode int) error {
	if response.StatusCode != statusCode {
		var errorResponse map[string]string
		var responseBytes []byte
		err := decodeResponse(response, &errorResponse)
		if err == nil {
			responseBytes, _ = json.Marshal(errorResponse)
		}
		return &statusCodeError{
			statusCode,
			response.StatusCode,
			string(responseBytes),
			err,
		}
	}
	return nil
}

func decodeResponse(response *nhttp.Response, v any) error {
	err := json.NewDecoder(response.Body).Decode(v)
	if err != nil {
		return fmt.Errorf("error decoding response: %w", err)
	}
	return nil
}

func (ta *testApp) createJob(ctx context.Context, jobData *data.JobRequestData) (model.JobId, error) {
	jobDataJson, err := json.Marshal(jobData)
	if err != nil {
		return 0, fmt.Errorf("error marshalling job data: %w", err)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	createJobRequest, _ := nhttp.NewRequestWithContext(
		timeoutCtx,
		"POST",
		url.CreateJob(),
		bytes.NewReader(jobDataJson),
	)
	createJobRequest.Header.Set("Content-Type", "application/json")

	response, err := ta.client.Do(createJobRequest)
	if err != nil {
		return 0, fmt.Errorf("error getting response while creating job %v: %w", jobData, err)
	}
	defer response.Body.Close()
	if err = checkStatusCode(response, nhttp.StatusOK); err != nil {
		return 0, fmt.Errorf("error creating job %v: %w", jobData, err)
	}
	var jobResponseId responseId
	if err = decodeResponse(response, &jobResponseId); err != nil {
		return 0, err
	}
	return jobResponseId.Id, nil
}

func (ta *testApp) getJob(ctx context.Context, url string) (*model.Job, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	getJobRequest, _ := nhttp.NewRequestWithContext(
		timeoutCtx,
		"GET",
		url,
		nil,
	)

	response, err := ta.client.Do(getJobRequest)
	if err != nil {
		return nil, fmt.Errorf("error getting response while getting job by url \"%s\": %w", url, err)
	}
	defer response.Body.Close()
	if err = checkStatusCode(response, nhttp.StatusOK); err != nil {
		return nil, fmt.Errorf("error getting job by url %s: %w", url, err)
	}
	var responseJob model.Job
	if err = decodeResponse(response, &responseJob); err != nil {
		return nil, err
	}
	return &responseJob, nil
}

func (ta *testApp) getJobById(ctx context.Context, id model.JobId) (*model.Job, error) {
	return ta.getJob(ctx, url.GetJobById(id))
}

func (ta *testApp) getJobByName(ctx context.Context, name string) (*model.Job, error) {
	return ta.getJob(ctx, url.GetJobByName(name))
}

func (ta *testApp) deleteJob(ctx context.Context, id model.JobId) error {
	deleteRequest, _ := nhttp.NewRequestWithContext(
		ctx,
		"DELETE",
		url.DeleteJob(id),
		nil,
	)
	response, err := ta.client.Do(deleteRequest)
	if err != nil {
		return fmt.Errorf("error getting response while deleting job with id %d: %w", id, err)
	}
	defer response.Body.Close()
	if err = checkStatusCode(response, nhttp.StatusOK); err != nil {
		return fmt.Errorf("error deleting job with id %d: %w", id, err)
	}
	return nil
}

func (ta *testApp) createJobs(ctx context.Context, t *testing.T) {
	for i := range data.InitialJobs {
		job := &data.InitialJobs[i]
		jobData := data.JobRequestData{
			Name:          job.Name,
			CrontabString: job.CrontabString,
			Command:       job.Command,
			Arguments:     job.Arguments,
			Timeout:       job.Timeout,
		}
		id, err := ta.createJob(ctx, &jobData)
		if err != nil {
			t.Fatal(fmt.Errorf("error creating initial jobs: %w", err))
		}
		job.Id = id
	}
}

func (ta *testApp) setupApp(ctx context.Context, t *testing.T) {
	ta.clearDatabase(ctx, t)
	ta.createJobs(ctx, t)
}

func resolveArguments() {
	_, testFilename, _, _ := runtime.Caller(0)
	scriptsDirectory := filepath.Join(filepath.Dir(testFilename), "test", "scripts")
	for i := range data.InitialJobs {
		scriptName := &data.InitialJobs[i].Arguments[0]
		*scriptName = filepath.Join(scriptsDirectory, *scriptName)
	}
}
