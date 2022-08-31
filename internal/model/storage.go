package model

import (
	"context"
	"errors"
	"time"
)

type JobId int64

type Job struct {
	Id            JobId         `json:"id"`
	Name          string        `json:"name"`
	CrontabString string        `json:"crontab_string"`
	ScriptPath    string        `json:"script_path"`
	Timeout       time.Duration `json:"timeout"`
}

var ErrorNotFound = errors.New("job not found")

type JobStorage interface {
	CreateJob(ctx context.Context, name, crontabString, scriptPath string, timeout time.Duration) (JobId, error)
	GetJob(ctx context.Context, id JobId) (Job, error)
	DeleteJob(ctx context.Context, id JobId) error
	GetJobByName(ctx context.Context, name string) (Job, error)
	MarkDueJobsRunning(ctx context.Context) ([]Job, error)
	MarkJobDone(ctx context.Context, job Job) error
}
