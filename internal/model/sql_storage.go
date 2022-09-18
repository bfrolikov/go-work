package model

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/robfig/cron/v3"
	"go-work/internal/model/sqlquery"
	"sync"
	"time"
)

type sqlJobStorage struct {
	database *sql.DB
	rwLock   *sync.RWMutex
}

func NewSQLJobStorage(ctx context.Context, driverName, dataSourceName string) (*sqlJobStorage, error) {
	database, err := sql.Open(driverName, dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("failed opening database: %w", err)
	}

	if err = database.PingContext(ctx); err != nil {
		database.Close()
		return nil, fmt.Errorf("failed checking database availibility: %w", err)
	}

	storage := sqlJobStorage{database, &sync.RWMutex{}}
	if err = storage.init(ctx); err != nil {
		database.Close()
		return nil, fmt.Errorf("failed initializing storage: %w", err)
	}
	return &storage, nil
}

func (st *sqlJobStorage) CreateJob(ctx context.Context, name, crontabString, command string, timeout time.Duration) (JobId, error) {
	schedule, err := cron.ParseStandard(crontabString)
	if err != nil {
		return 0, fmt.Errorf("failed parsing crontab string \"%s\" while creating job: %w", crontabString, err)
	}

	var id JobId
	transactionFunc := func(ctx context.Context, tx *sql.Tx) error {
		err := tx.QueryRowContext(
			ctx,
			sqlquery.NewJob,
			name,
			crontabString,
			command,
			timeout,
			schedule.Next(time.Now()),
		).Scan(&id)
		if err != nil {
			err = fmt.Errorf("failed scanning job: %w", err)
		}
		return err
	}

	if err = st.transact(ctx, transactionFunc); err != nil {
		return 0, fmt.Errorf("failed creating job: %w", err)
	}

	return id, nil
}

func (st *sqlJobStorage) GetJob(ctx context.Context, id JobId) (Job, error) {
	job, err := st.getJobBy(ctx, sqlquery.GetJob, id)
	if err != nil {
		err = fmt.Errorf("failed getting job by id %d: %w", id, err)
	}
	return job, err
}

func (st *sqlJobStorage) DeleteJob(ctx context.Context, id JobId) error {
	err := st.updateJobs(ctx, sqlquery.DeleteJob, id)
	if err != nil {
		err = fmt.Errorf("failed deleting job with id %d: %w", id, err)
	}
	return err
}

func (st *sqlJobStorage) GetJobByName(ctx context.Context, name string) (Job, error) {
	job, err := st.getJobBy(ctx, sqlquery.GetJobByName, name)
	if err != nil {
		err = fmt.Errorf("failed getting job by name %s: %w", name, err)
	}
	return job, err
}

func (st *sqlJobStorage) MarkDueJobsRunning(ctx context.Context) ([]Job, error) {
	jobs := make([]Job, 0)
	transactionFunc := func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, sqlquery.MarkDueJobsRunning, time.Now())
		if err != nil {
			return fmt.Errorf("failed mark due jobs running query: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			job := Job{}
			err := scanJob(rows, &job)
			if err != nil {
				return fmt.Errorf("failed scanning job: %w", err)
			}
			jobs = append(jobs, job)
		}
		return rows.Close()
	}

	if err := st.transact(ctx, transactionFunc); err != nil {
		return nil, fmt.Errorf("failed marking due jobs running: %w", err)
	}
	return jobs, nil
}

func (st *sqlJobStorage) MarkJobDone(ctx context.Context, job Job) error {
	schedule, err := cron.ParseStandard(job.CrontabString)
	if err != nil {
		return fmt.Errorf("failed parsing crontab string %s while marking job done: %w", job.CrontabString, err)
	}

	err = st.updateJobs(ctx, sqlquery.MarkDone, schedule.Next(time.Now()), job.Id)
	if err != nil {
		err = fmt.Errorf("failed marking job done: %w", err)
	}
	return err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanJob(sc scanner, job *Job) error {
	return sc.Scan(
		&job.Id,
		&job.Name,
		&job.CrontabString,
		&job.Command,
		&job.Timeout,
	)
}

func (st *sqlJobStorage) getJobBy(ctx context.Context, query string, params ...any) (Job, error) {
	st.rwLock.RLock()
	defer st.rwLock.RUnlock()

	job := Job{}
	err := scanJob(st.database.QueryRowContext(ctx, query, params...), &job)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Job{}, ErrorNotFound
		}
		return Job{}, err
	}
	return job, nil
}

func (st *sqlJobStorage) updateJobs(ctx context.Context, query string, params ...any) error {
	st.rwLock.Lock()
	defer st.rwLock.Unlock()

	_, err := st.database.ExecContext(ctx, query, params...)
	return err
}

func (st *sqlJobStorage) transact(ctx context.Context, transactionFunc func(context.Context, *sql.Tx) error) error {
	st.rwLock.Lock()
	defer st.rwLock.Unlock()

	tx, err := st.database.BeginTx(ctx, nil)
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

func (st *sqlJobStorage) init(ctx context.Context) error {
	transactionFunc := func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, sqlquery.ResetState)
		if err != nil {
			return fmt.Errorf("error resetting storage state: %w", err)
		}

		for {
			rows, err := tx.QueryContext(ctx, sqlquery.FindNullNextExecutionTime)
			if err != nil {
				return fmt.Errorf("error finding jobs with null next execution time: %w", err)
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
				return fmt.Errorf("error scanning jobs: %w", err)
			}
			if closeError != nil {
				return closeError
			}

			for _, job := range jobs {
				schedule, err := cron.ParseStandard(job.CrontabString)
				if err != nil {
					return fmt.Errorf("error parsing crontab string %s: %w", job.CrontabString, err)
				}
				_, err = tx.ExecContext(ctx, sqlquery.SetNextExecutionTime, schedule.Next(time.Now()), job.Id)
				if err != nil {
					return fmt.Errorf("error setting next execution time for job with id %d: %w", job.Id, err)
				}
			}
		}
		return nil
	}
	return st.transact(ctx, transactionFunc)
}
