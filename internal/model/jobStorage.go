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
	nextExecutionTime time.Time
	running           bool
}

type JobStorage interface {
	CreateJob(ctx context.Context, name, crontabString, scriptPath string, timeout time.Duration) (JobId, error)
	GetJob(ctx context.Context, id JobId) (Job, error)
	DeleteJob(ctx context.Context, id JobId) error
	GetJobByName(ctx context.Context, name string) (Job, error)
	FindDueJobs(ctx context.Context) ([]Job, error)
	MarkJobRunning(ctx context.Context, job Job) error
	MarkJobDone(ctx context.Context, job Job) error
}
