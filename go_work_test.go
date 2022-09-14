package go_work

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	_ "github.com/lib/pq"
	"go-work/internal/http"
	"go-work/internal/model"
	"go-work/test/data"
	"go-work/test/sqlquery"
	nhttp "net/http"
	"os"
	"testing"
	"time"
)

type testApp struct {
	server   *nhttp.Server
	client   *nhttp.Client
	database *sql.DB
}

const timeout = time.Second * 30

func TestGoWork(t *testing.T) {
	background := context.Background()
	dataSourceName := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("TEST_DB_HOST"),
		os.Getenv("TEST_DB_PORT"),
		os.Getenv("TEST_DB_USER "),
		os.Getenv("TEST_DB_PASSWORD"),
		os.Getenv("TEST_DB_NAME"),
	)
	database, err := sql.Open("postgres", dataSourceName)
	if err != nil {
		t.Fatal(fmt.Errorf("error opening database: %w", err))
	}
	storage, err := model.NewSQLJobStorage(background, "postgres", dataSourceName)
	if err != nil {
		t.Fatal(fmt.Errorf("could not create job storage: %w", err))
	}
	server, err := http.NewJobServer(storage, os.Getenv("TEST_SERVER_PORT"))
	if err != nil {
		t.Fatal(fmt.Errorf("could not create job server: %w", err))
	}

	app := testApp{server, nhttp.DefaultClient, database}
	t.Run("Test REST API", func(t *testing.T) {
		t.Run("Test getting job by id", func(t *testing.T) {
			app.setupApp(background, t)

			existingJob := data.InitialJobs[0]
			job, err := app.getJobById(existingJob.Id)
			if err != nil {
				t.Fatal(fmt.Errorf("error getting created job by id %d: %w", existingJob.Id, err))
			}
			requireEqual("name", existingJob.Name, job.Name, t)
			requireEqual("crontabString", existingJob.CrontabString, job.CrontabString, t)
			requireEqual("scriptPath", existingJob.ScriptPath, job.ScriptPath, t)
			requireEqual("timeout", existingJob.Timeout, job.Timeout, t)
		})

		t.Run("Test creating and getting new job by name", func(t *testing.T) {
			app.setupApp(background, t)

			existingJob := data.InitialJobs[0]
			job, err := app.getJobByName(existingJob.Name)
			if err != nil {
				t.Fatal(fmt.Errorf("error getting created job by name %s: %w", existingJob.Name, err))
			}
			requireEqual("name", existingJob.Name, job.Name, t)
			requireEqual("crontabString", existingJob.CrontabString, job.CrontabString, t)
			requireEqual("scriptPath", existingJob.ScriptPath, job.ScriptPath, t)
			requireEqual("timeout", existingJob.Timeout, job.Timeout, t)
		})

		t.Run("Test deleting job", func(t *testing.T) {
			app.setupApp(background, t)

			existingJob := data.InitialJobs[0]
			err := app.deleteJob(existingJob.Id)
			if err != nil {
				t.Fatal(fmt.Errorf("error deleting job with id %d: %w", existingJob.Id, err))
			}
		})

		t.Run("Test creating already existing job", func(t *testing.T) {
			app.setupApp(background, t)

			existingJob := data.InitialJobs[0]
			existingJobData := data.JobData{
				Name:          existingJob.Name,
				CrontabString: existingJob.CrontabString,
				ScriptPath:    existingJob.ScriptPath,
				Timeout:       existingJob.Timeout,
			}
			_, err := app.createJob(&existingJobData)
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
			_, err := app.getJobById(maxId + 1)
			expectErrorStatusCode(err, nhttp.StatusNotFound, t)
		})
	})

	//TODO: check cron functionality
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
	_, err := ta.database.ExecContext(timeoutCtx, sqlquery.DeleteAll)
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
}

func (e *statusCodeError) Error() string {
	return fmt.Sprintf(
		"expected status code %d, got %d",
		e.expectedStatusCode,
		e.receivedStatusCode,
	)
}

func checkStatusCode(response *nhttp.Response, statusCode int) error {
	if response.StatusCode != statusCode {
		return &statusCodeError{statusCode, response.StatusCode}
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

func (ta *testApp) createJob(jobData *data.JobData) (model.JobId, error) {
	createJobUrl := fmt.Sprintf("http://localhost:%s/api/v1/job", os.Getenv("TEST_SERVER_PORT"))
	jobDataJson, err := json.Marshal(jobData)
	if err != nil {
		return 0, fmt.Errorf("error marshalling job data: %w", err)
	}
	response, err := ta.client.Post(
		createJobUrl,
		"application/json",
		bytes.NewReader(jobDataJson),
	)
	if err != nil {
		return 0, fmt.Errorf("error getting response while creating job %v: %w", jobData, err)
	}
	defer response.Body.Close()
	if err = checkStatusCode(response, nhttp.StatusOK); err != nil {
		return 0, err
	}
	var jobResponseId responseId
	if err = decodeResponse(response, &jobResponseId); err != nil {
		return 0, err
	}
	return jobResponseId.Id, nil
}

func (ta *testApp) getJob(url string) (*model.Job, error) {
	response, err := ta.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error getting response while getting job by url \"%s\": %w", url, err)
	}
	defer response.Body.Close()
	if err = checkStatusCode(response, nhttp.StatusOK); err != nil {
		return nil, err
	}
	var responseJob model.Job
	if err = decodeResponse(response, &responseJob); err != nil {
		return nil, err
	}
	return &responseJob, nil
}

func (ta *testApp) getJobById(id model.JobId) (*model.Job, error) {
	getJobByIdUrl := fmt.Sprintf("http://localhost:%s/api/v1/job/%d/", os.Getenv("TEST_SERVER_PORT"), id)
	return ta.getJob(getJobByIdUrl)
}

func (ta *testApp) getJobByName(name string) (*model.Job, error) {
	getJobByNameUrl := fmt.Sprintf("http://localhost:%s/api/v1/job/%s/", os.Getenv("TEST_SERVER_PORT"), name)
	return ta.getJob(getJobByNameUrl)
}

func (ta *testApp) deleteJob(id model.JobId) error {
	deleteJobUrl := fmt.Sprintf("http://localhost:%s/api/v1/job/%s/", os.Getenv("TEST_SERVER_PORT"), id)
	deleteRequest, _ := nhttp.NewRequest("DELETE", deleteJobUrl, nil)
	response, err := ta.client.Do(deleteRequest)
	if err != nil {
		return fmt.Errorf("error getting response while deleting job with id %d: %w", id, err)
	}
	defer response.Body.Close()
	return checkStatusCode(response, nhttp.StatusOK)
}

func (ta *testApp) createJobs(t *testing.T) {
	for i := range data.InitialJobs {
		job := &data.InitialJobs[i]
		jobData := data.JobData{
			Name:          job.Name,
			CrontabString: job.CrontabString,
			ScriptPath:    job.ScriptPath,
			Timeout:       job.Timeout,
		}
		id, err := ta.createJob(&jobData)
		if err != nil {
			t.Fatal(fmt.Errorf("error creating initial jobs: %w", err))
		}
		job.Id = id
	}
}

func (ta *testApp) setupApp(ctx context.Context, t *testing.T) {
	ta.clearDatabase(ctx, t)
	ta.createJobs(t)
}
