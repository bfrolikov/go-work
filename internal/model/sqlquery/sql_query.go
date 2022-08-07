package sqlquery

import "time"

const (
	NewJob                    = "INSERT INTO jobs (name, crontabString, scriptPath, timeout, nextExecutionTime) values ($1, $2, $3, $4, $5) RETURNING id"
	GetJob                    = "SELECT id, name, crontabString, scriptPath, timeout FROM jobs WHERE id = $1"
	DeleteJob                 = "DELETE FROM jobs WHERE id = $1"
	GetJobByName              = "SELECT id, name, crontabString, scriptPath, timeout FROM jobs WHERE name = $1"
	MarkDueJobsRunning        = "UPDATE jobs SET running = true WHERE nextExecutionTime <= $1 AND not running RETURNING id, name, crontabString, scriptPath, timeout"
	MarkDone                  = "UPDATE jobs SET nextExecutionTime = $1, running = false WHERE id = $2"
	ResetState                = "UPDATE jobs SET nextExecutionTime = NULL, running = false"
	FindNullNextExecutionTime = "SELECT id, name, crontabString, scriptPath, timeout FROM jobs WHERE nextExecutionTime IS NULL LIMIT 100"
	SetNextExecutionTime      = "UPDATE jobs SET nextExecutionTime = $1 WHERE id = $2"
	DatabaseOperationTimeout  = time.Second * 5
)
