package model

import (
	"context"
	"database/sql"
	"github.com/robfig/cron/v3"
	"go-work/internal/model/sqlquery"
	"sync"
	"time"
)

type sqlJobStorage struct {
	database *sql.DB
	rwLock   *sync.RWMutex
}

func NewSQLJobStorage(ctx context.Context, driverName, dataSourceName string) (sqlJobStorage, error) {
	database, err := sql.Open(driverName, dataSourceName)
	if err != nil {
		return sqlJobStorage{}, err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, sqlquery.DatabaseOperationTimeout)
	defer cancel()
	if err = database.PingContext(timeoutCtx); err != nil {
		database.Close()
		return sqlJobStorage{}, err
	}

	storage := sqlJobStorage{database, &sync.RWMutex{}}
	if err = storage.init(ctx); err != nil {
		database.Close()
		return sqlJobStorage{}, err
	}
	return storage, nil
}

func (st sqlJobStorage) CreateJob(ctx context.Context, name, crontabString, scriptPath string, timeout time.Duration) (JobId, error) {
	schedule, err := cron.ParseStandard(crontabString)
	if err != nil {
		return 0, err
	}

	var id JobId
	transactionFunc := func(ctx context.Context, tx *sql.Tx) error {
		processFunc := func(row *sql.Row) error {
			return row.Scan(&id)
		}
		return queryRowTimeout(
			ctx,
			tx,
			sqlquery.NewJob,
			processFunc,
			name,
			crontabString,
			scriptPath,
			timeout,
			schedule.Next(time.Now()),
		)
	}

	if err = st.transact(ctx, transactionFunc); err != nil {
		return 0, err
	}

	return id, nil
}

func (st sqlJobStorage) GetJob(ctx context.Context, id JobId) (Job, error) {
	return st.getJobBy(ctx, sqlquery.GetJob, id)
}

func (st sqlJobStorage) DeleteJob(ctx context.Context, id JobId) error {
	return st.updateJobs(ctx, sqlquery.DeleteJob, id)
}

func (st sqlJobStorage) GetJobByName(ctx context.Context, name string) (Job, error) {
	return st.getJobBy(ctx, sqlquery.GetJobByName, name)
}

func (st sqlJobStorage) MarkDueJobsRunning(ctx context.Context) ([]Job, error) {
	jobs := make([]Job, 0)
	transactionFunc := func(ctx context.Context, tx *sql.Tx) error {
		processFunc := func(rows *sql.Rows) error {
			for rows.Next() {
				job := Job{}
				err := scanJob(rows, &job)
				if err != nil {
					return err
				}
				jobs = append(jobs, job)
			}
			return nil
		}

		return queryTimeout(ctx, tx, sqlquery.MarkDueJobsRunning, processFunc, time.Now())
	}

	if err := st.transact(ctx, transactionFunc); err != nil {
		return nil, err
	}
	return jobs, nil
}

func (st sqlJobStorage) MarkJobDone(ctx context.Context, job Job) error {
	schedule, err := cron.ParseStandard(job.CrontabString)
	if err != nil {
		return err
	}

	return st.updateJobs(ctx, sqlquery.MarkDone, schedule.Next(time.Now()), job.Id)
}

type scanner interface {
	Scan(dest ...any) error
}

func scanJob(sc scanner, job *Job) error {
	return sc.Scan(
		&job.Id,
		&job.Name,
		&job.CrontabString,
		&job.ScriptPath,
		&job.Timeout,
	)
}

type contextQueryRunner interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func execTimeout(ctx context.Context, runner contextQueryRunner, query string, args ...any) (sql.Result, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, sqlquery.DatabaseOperationTimeout)
	defer cancel()
	return runner.ExecContext(timeoutCtx, query, args...)
}

func queryRowTimeout(
	ctx context.Context,
	runner contextQueryRunner,
	query string,
	processFunc func(*sql.Row) error,
	args ...any,
) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, sqlquery.DatabaseOperationTimeout)
	defer cancel()
	return processFunc(runner.QueryRowContext(timeoutCtx, query, args...))
}

func queryTimeout(
	ctx context.Context,
	runner contextQueryRunner,
	query string,
	processFunc func(*sql.Rows) error,
	args ...any,
) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, sqlquery.DatabaseOperationTimeout)
	defer cancel()
	rows, err := runner.QueryContext(timeoutCtx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	err = processFunc(rows)
	if err != nil {
		return err
	}
	return rows.Close()
}

func (st sqlJobStorage) getJobBy(ctx context.Context, query string, params ...any) (Job, error) {
	job := Job{}
	processFunc := func(row *sql.Row) error {
		return scanJob(row, &job)
	}

	st.rwLock.RLock()
	err := queryRowTimeout(ctx, st.database, query, processFunc, params...)
	st.rwLock.RUnlock()

	if err != nil {
		return Job{}, err
	}
	return job, nil
}

func (st sqlJobStorage) updateJobs(ctx context.Context, query string, params ...any) error {
	st.rwLock.Lock()
	defer st.rwLock.Unlock()

	_, err := execTimeout(ctx, st.database, query, params...)

	return err
}

func (st sqlJobStorage) transact(ctx context.Context, transactionFunc func(context.Context, *sql.Tx) error) error {
	st.rwLock.Lock()
	defer st.rwLock.Unlock()

	tx, err := st.database.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = transactionFunc(ctx, tx)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (st sqlJobStorage) init(ctx context.Context) error {
	transactionFunc := func(ctx context.Context, tx *sql.Tx) error {
		_, err := execTimeout(ctx, tx, sqlquery.ResetState)
		if err != nil {
			return err
		}

		done := false
		for !done {
			jobs := make([]Job, 0)
			fetchJobs := func(rows *sql.Rows) error {
				if !rows.Next() {
					done = true
					return nil
				}
				for {
					job := Job{}
					if err := scanJob(rows, &job); err != nil {
						return err
					}
					jobs = append(jobs, job)
					if !rows.Next() {
						break
					}
				}
				return nil
			}
			err = queryTimeout(ctx, tx, sqlquery.FindNullNextExecutionTime, fetchJobs)
			if err != nil {
				return err
			}

			for _, job := range jobs {
				schedule, err := cron.ParseStandard(job.CrontabString)
				if err != nil {
					return err
				}
				_, err = execTimeout(ctx, tx, sqlquery.SetNextExecutionTime, schedule.Next(time.Now()), job.Id)
				if err != nil {
					return err
				}
			}
		}
		return nil
	}
	return st.transact(ctx, transactionFunc)
}
