package model

import (
	"context"
	"time"
)

type JobId int64

type Job struct {
	Id                JobId
	Name              string
	CrontabString     string
	ScriptPath        string
	Timeout           time.Duration
}

type JobStorage interface {
	CreateJob(ctx context.Context, name, crontabString, scriptPath string, timeout time.Duration) (JobId, error)
	GetJob(ctx context.Context, id JobId) (Job, error)
	DeleteJob(ctx context.Context, id JobId) error
	GetJobByName(ctx context.Context, name string) (Job, error)
	MarkDueJobsRunning(ctx context.Context) ([]Job, error)
	MarkJobDone(ctx context.Context, job Job) error
}
