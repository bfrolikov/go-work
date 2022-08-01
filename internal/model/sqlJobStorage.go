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
	findDueQuery             = "SELECT * FROM jobs WHERE nextExecutionTime <= $1 AND not running"
	markRunningQuery         = "UPDATE jobs SET running = true WHERE id = $1"
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
	timeoutCtx, cancel := context.WithTimeout(ctx, databaseOperationTimeout)
	defer cancel()

	var id JobId
	storage.rwLock.Lock()
	err = storage.database.QueryRowContext(
		timeoutCtx,
		newJobQuery,
		name,
		crontabString,
		scriptPath,
		timeout,
		schedule.Next(time.Now()),
	).Scan(&id)
	storage.rwLock.Unlock()

	if err != nil {
		return 0, err
	}

	return id, nil
}

func (storage SQLJobStorage) FindDueJobs(ctx context.Context) ([]Job, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, databaseOperationTimeout)
	defer cancel()

	storage.rwLock.RLock()
	rows, err := storage.database.QueryContext(timeoutCtx, findDueQuery, time.Now())
	storage.rwLock.RUnlock()

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	dueJobs := make([]Job, 0)
	for rows.Next() {
		var dueJob Job
		err = rows.Scan(
			&dueJob.Id,
			&dueJob.Name,
			&dueJob.CrontabString,
			&dueJob.ScriptPath,
			&dueJob.Timeout,
			&dueJob.nextExecutionTime,
			&dueJob.running,
		)
		if err != nil {
			return nil, err
		}
		dueJobs = append(dueJobs, dueJob)
		//TODO: cancel on ctx.Done()
	}

	err = rows.Close()
	if err != nil {
		return nil, err
	}

	return dueJobs, nil
}

func (storage SQLJobStorage) MarkJobRunning(ctx context.Context, job Job) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, databaseOperationTimeout)
	defer cancel()

	storage.rwLock.Lock()
	_, err := storage.database.ExecContext(timeoutCtx, markRunningQuery, job.Id)
	storage.rwLock.Unlock()

	return err
}

func (storage SQLJobStorage) MarkJobDone(ctx context.Context, job Job) error {
	schedule, err := cron.ParseStandard(job.CrontabString)
	if err != nil {
		return err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, databaseOperationTimeout)
	defer cancel()

	storage.rwLock.Lock()
	_, err = storage.database.ExecContext(timeoutCtx, markDoneQuery, schedule.Next(time.Now()), job.Id)
	storage.rwLock.Unlock()

	return err
}
