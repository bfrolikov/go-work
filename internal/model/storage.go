package model

import (
	"context"
	"errors"
)

type JobId int64

type Job struct {
	Id            JobId    `json:"id"`
	Name          string   `json:"name"`
	CrontabString string   `json:"crontabString"`
	Command       string   `json:"command"`
	Arguments     []string `json:"arguments,omitempty"`
	Timeout       uint     `json:"timeout"`
}

var ErrorNotFound = errors.New("job not found")

type JobStorage interface {
	CreateJob(ctx context.Context, name, crontabString, command string, arguments []string, timeout uint) (JobId, error)
	GetJob(ctx context.Context, id JobId) (*Job, error)
	DeleteJob(ctx context.Context, id JobId) error
	GetJobByName(ctx context.Context, name string) (*Job, error)
	MarkDueJobsRunning(ctx context.Context) ([]*Job, error)
	MarkJobDone(ctx context.Context, job *Job) error
}
