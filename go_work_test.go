package go_work

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
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

type testSuite struct {
	t   *testing.T
	app *testApp
}

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
		suite := testSuite{
			t,
			&app,
		}

		suite.setupApp(background)
		t.Run("Test creating and getting new job by id", func(t *testing.T) {
			id, err := suite.createJob(&data.CreatedJob)
			if err != nil {
				t.Fatal(fmt.Errorf("error creating new job: %w", err))
			}
			job, err := suite.getJobById(id)
			if err != nil {
				t.Fatal(fmt.Errorf("error getting created job by id %d: %w", id, err))
			}
			requireEqual("name", &data.CreatedJob.Name, &job.Name, t)
			requireEqual("crontabString", &data.CreatedJob.CrontabString, &job.CrontabString, t)
			requireEqual("scriptPath", &data.CreatedJob.ScriptPath, &job.ScriptPath, t)
			requireEqual("timeout", &data.CreatedJob.Timeout, &job.Timeout, t)
		}) //TODO: delete, create invalid, get nonexistent

		suite.setupApp(background)
		t.Run("Test creating and getting new job by name", func(t *testing.T) {
			_, err := suite.createJob(&data.CreatedJob)
			if err != nil {
				t.Fatal(fmt.Errorf("error creating new job: %w", err))
			}
			job, err := suite.getJobByName(data.CreatedJob.Name)
			if err != nil {
				t.Fatal(fmt.Errorf("error getting created job by name %s: %w", data.CreatedJob.Name, err))
			}
			requireEqual("name", &data.CreatedJob.Name, &job.Name, t)
			requireEqual("crontabString", &data.CreatedJob.CrontabString, &job.CrontabString, t)
			requireEqual("scriptPath", &data.CreatedJob.ScriptPath, &job.ScriptPath, t)
			requireEqual("timeout", &data.CreatedJob.Timeout, &job.Timeout, t)
		})

		suite.setupApp(background)
		t.Run("Test deleting job", func(t *testing.T) {

		})
	})

	//TODO: check cron functionality
}

func requireEqual[K comparable](name string, first K, second K, t *testing.T) {
	if first != second {
		t.Fatalf("expected %s to be equal, instead got %v and %v", name, first, second)
	}
}

func (ts *testSuite) clearDatabase(ctx context.Context) {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	_, err := ts.app.database.ExecContext(timeoutCtx, sqlquery.DeleteAll)
	if err != nil {
		ts.t.Fatal(fmt.Errorf("error clearing database %w", err))
	}
}

type responseId struct {
	Id model.JobId `json:"id"`
}

func (ts *testSuite) expectStatusCode(response *nhttp.Response, statusCode int) {
	if response.StatusCode != statusCode {
		ts.t.Fatalf("expected status code %d, got %d", statusCode, response.StatusCode)
	}
}

func decodeResponse(response *nhttp.Response, v any) error {
	err := json.NewDecoder(response.Body).Decode(v)
	if err != nil {
		return fmt.Errorf("error decoding response: %w", err)
	}
	return nil
}

func (ts *testSuite) createJob(jobData *data.JobData) (model.JobId, error) {
	createJobUrl := fmt.Sprintf("http://localhost:%s/api/v1/job", os.Getenv("TEST_SERVER_PORT"))
	jobDataJson, err := json.Marshal(jobData)
	if err != nil {
		return 0, fmt.Errorf("error marshalling job data: %w", err)
	}
	response, err := ts.app.client.Post(
		createJobUrl,
		"application/json",
		bytes.NewReader(jobDataJson),
	)
	if err != nil {
		return 0, fmt.Errorf("error getting response: %w", err)
	}
	defer response.Body.Close()
	ts.expectStatusCode(response, nhttp.StatusOK)
	var jobResponseId responseId
	if err = decodeResponse(response, &jobResponseId); err != nil {
		return 0, err
	}
	return jobResponseId.Id, nil
}

func (ts *testSuite) getJob(url string) (*model.Job, error) {
	response, err := ts.app.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	ts.expectStatusCode(response, nhttp.StatusOK)
	var responseJob model.Job
	if err = decodeResponse(response, &responseJob); err != nil {
		return nil, err
	}
	return &responseJob, nil
}

func (ts *testSuite) getJobById(id model.JobId) (*model.Job, error) {
	getJobByIdUrl := fmt.Sprintf("http://localhost:%s/api/v1/job/%d/", os.Getenv("TEST_SERVER_PORT"), id)
	return ts.getJob(getJobByIdUrl)
}

func (ts *testSuite) getJobByName(name string) (*model.Job, error) {
	getJobByNameUrl := fmt.Sprintf("http://localhost:%s/api/v1/job/%s/", os.Getenv("TEST_SERVER_PORT"), name)
	return ts.getJob(getJobByNameUrl)
}

func (ts *testSuite) deleteJob() error {
	return nil
}

func (ts *testSuite) createJobs() {
	for _, jobData := range data.InitialJobs {
		_, err := ts.createJob(&jobData)
		if err != nil {
			ts.t.Fatal(fmt.Errorf("error creating initial jobs: %w", err))
		}
	}
}

func (ts *testSuite) setupApp(ctx context.Context) {
	ts.clearDatabase(ctx)
	ts.createJobs()
}
