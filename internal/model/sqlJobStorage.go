package model

import (
	"context"
	"database/sql"
	"github.com/robfig/cron/v3"
	"sync"
	"time"
)

const (
	newJobQuery              = "INSERT INTO jobs (name, crontabString, scriptPath, timeout, nextExecutionTime) values ($1, $2, $3, $4, $5) RETURNING id"
	getJobQuery              = "SELECT * FROM jobs WHERE id = $1"
	deleteJobQuery           = "DELETE FROM jobs WHERE id = $1"
	getJobByNameQuery        = "SELECT * FROM jobs WHERE name = $1"
	markDueJobsRunningQuery  = "UPDATE jobs SET running = true WHERE nextExecutionTime <= $1 AND not running RETURNING *"
	markDoneQuery            = "UPDATE jobs SET nextExecutionTime = $1, running = false WHERE id = $2"
	databaseOperationTimeout = time.Second
)

type SQLJobStorage struct {
	database *sql.DB
	rwLock   *sync.RWMutex
}

func NewSQLJobStorage(ctx context.Context, driverName, dataSourceName string) (SQLJobStorage, error) {
	database, err := sql.Open(driverName, dataSourceName)
	if err != nil {
		return SQLJobStorage{}, err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, databaseOperationTimeout)
	defer cancel()
	if err = database.PingContext(timeoutCtx); err != nil {
		database.Close()
		return SQLJobStorage{}, err
	}

	return SQLJobStorage{database, &sync.RWMutex{}}, nil
}

func (storage SQLJobStorage) CreateJob(ctx context.Context, name, crontabString, scriptPath string, timeout time.Duration) (JobId, error) {
	schedule, err := cron.ParseStandard(crontabString)
	if err != nil {
		return 0, err
	}

	storage.rwLock.Lock()
	defer storage.rwLock.Unlock()

	txTimeoutCtx, cancel := context.WithTimeout(ctx, databaseOperationTimeout)
	defer cancel()
	tx, err := storage.database.BeginTx(txTimeoutCtx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	queryTimeoutCtx, cancel := context.WithTimeout(ctx, databaseOperationTimeout)
	defer cancel()
	var id JobId
	err = tx.QueryRowContext(
		queryTimeoutCtx,
		newJobQuery,
		name,
		crontabString,
		scriptPath,
		timeout,
		schedule.Next(time.Now()),
	).Scan(&id)
	if err != nil {
		return 0, err
	}

	if err = tx.Commit(); err != nil {
		return 0, err
	}

	return id, nil
}

func (storage SQLJobStorage) GetJob(ctx context.Context, id JobId) (Job, error) {
	return storage.getJobBy(ctx, getJobQuery, id)
}

func (storage SQLJobStorage) DeleteJob(ctx context.Context, id JobId) error {
	storage.rwLock.Lock()
	defer storage.rwLock.Unlock()

	timeoutCtx, cancel := context.WithTimeout(ctx, databaseOperationTimeout)
	defer cancel()
	_, err := storage.database.ExecContext(timeoutCtx, deleteJobQuery, id)

	return err
}

func (storage SQLJobStorage) GetJobByName(ctx context.Context, name string) (Job, error) {
	return storage.getJobBy(ctx, getJobByNameQuery, name)
}

func (storage SQLJobStorage) MarkDueJobsRunning(ctx context.Context) ([]Job, error) {
	storage.rwLock.Lock()
	defer storage.rwLock.Unlock()

	txTimeoutCtx, cancel := context.WithTimeout(ctx, databaseOperationTimeout)
	defer cancel()
	tx, err := storage.database.BeginTx(txTimeoutCtx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	queryTimeoutCtx, cancel := context.WithTimeout(ctx, databaseOperationTimeout)
	defer cancel()
	rows, err := tx.QueryContext(queryTimeoutCtx, markDueJobsRunningQuery, time.Now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := make([]Job, 0)
	for rows.Next() {
		var job Job
		err = scanJob(rows, &job)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return jobs, nil

}

func (storage SQLJobStorage) MarkJobDone(ctx context.Context, job Job) error {
	schedule, err := cron.ParseStandard(job.CrontabString)
	if err != nil {
		return err
	}

	return storage.updateJobs(ctx, markDoneQuery, schedule.Next(time.Now()), job.Id)
}

type scanable interface {
	Scan(dest ...any) error
}

func scanJob(rowToScan scanable, job *Job) error {
	return rowToScan.Scan(
		&job.Id,
		&job.Name,
		&job.CrontabString,
		&job.ScriptPath,
		&job.Timeout,
		&job.nextExecutionTime,
		&job.running,
	)
}

func (storage SQLJobStorage) getJobBy(ctx context.Context, query string, params ...any) (Job, error) {
	storage.rwLock.RLock()
	timeoutCtx, cancel := context.WithTimeout(ctx, databaseOperationTimeout)
	defer cancel()
	row := storage.database.QueryRowContext(timeoutCtx, query, params)
	storage.rwLock.RUnlock()

	var job Job
	err := scanJob(row, &job)
	if err != nil {
		return Job{}, err
	}
	return job, nil
}

func (storage SQLJobStorage) updateJobs(ctx context.Context, query string, params ...any) error {
	storage.rwLock.Lock()
	defer storage.rwLock.Unlock()

	timeoutCtx, cancel := context.WithTimeout(ctx, databaseOperationTimeout)
	defer cancel()
	_, err := storage.database.ExecContext(timeoutCtx, query, params)

	return err
}
