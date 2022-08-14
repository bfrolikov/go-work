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
		return tx.QueryRowContext(
			ctx,
			sqlquery.NewJob,
			name,
			crontabString,
			scriptPath,
			timeout,
			schedule.Next(time.Now()),
		).Scan(&id)
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
		rows, err := tx.QueryContext(ctx, sqlquery.MarkDueJobsRunning, time.Now())
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			job := Job{}
			err := scanJob(rows, &job)
			if err != nil {
				return err
			}
			jobs = append(jobs, job)
		}
		return rows.Close()
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

func (st sqlJobStorage) getJobBy(ctx context.Context, query string, params ...any) (Job, error) {
	st.rwLock.RLock()
	defer st.rwLock.RUnlock()

	job := Job{}
	timeoutCtx, cancel := context.WithTimeout(ctx, sqlquery.DatabaseOperationTimeout)
	defer cancel()
	err := scanJob(st.database.QueryRowContext(timeoutCtx, query, params...), &job)
	if err != nil {
		return Job{}, err
	}
	return job, nil
}

func (st sqlJobStorage) updateJobs(ctx context.Context, query string, params ...any) error {
	st.rwLock.Lock()
	defer st.rwLock.Unlock()

	timeoutCtx, cancel := context.WithTimeout(ctx, sqlquery.DatabaseOperationTimeout)
	defer cancel()
	_, err := st.database.ExecContext(timeoutCtx, query, params...)
	return err
}

func (st sqlJobStorage) transact(ctx context.Context, transactionFunc func(context.Context, *sql.Tx) error) error {
	st.rwLock.Lock()
	defer st.rwLock.Unlock()

	timeoutCtx, cancel := context.WithTimeout(ctx, sqlquery.DatabaseOperationTimeout)
	defer cancel()
	tx, err := st.database.BeginTx(timeoutCtx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = transactionFunc(timeoutCtx, tx)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (st sqlJobStorage) init(ctx context.Context) error {
	transactionFunc := func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, sqlquery.ResetState)
		if err != nil {
			return err
		}

		for {
			rows, err := tx.QueryContext(ctx, sqlquery.FindNullNextExecutionTime)
			if err != nil {
				return err
			}
			if !rows.Next() {
				break
			}

			jobs := make([]Job, 0)
			for {
				job := Job{}
				if err := scanJob(rows, &job); err != nil {
					break
				}
				jobs = append(jobs, job)
				if !rows.Next() {
					break
				}
			}
			closeError := rows.Close()
			if err = rows.Err(); err != nil {
				return err
			}
			if closeError != nil {
				return closeError
			}

			for _, job := range jobs {
				schedule, err := cron.ParseStandard(job.CrontabString)
				if err != nil {
					return err
				}
				_, err = tx.ExecContext(ctx, sqlquery.SetNextExecutionTime, schedule.Next(time.Now()), job.Id)
				if err != nil {
					return err
				}
			}
		}
		return nil
	}
	return st.transact(ctx, transactionFunc)
}
