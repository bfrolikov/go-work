package sqlquery

const (
	NewJob                    = "INSERT INTO jobs (name, crontabString, command, arguments, timeout, nextExecutionTime) values ($1, $2, $3, $4, $5, $6) RETURNING id"
	GetJob                    = "SELECT id, name, crontabString, command, arguments, timeout FROM jobs WHERE id = $1"
	DeleteJob                 = "DELETE FROM jobs WHERE id = $1"
	GetJobByName              = "SELECT id, name, crontabString, command, arguments, timeout FROM jobs WHERE name = $1"
	MarkDueJobsRunning        = "UPDATE jobs SET running = true WHERE nextExecutionTime <= $1 AND not running RETURNING id, name, crontabString, command, arguments, timeout"
	MarkDone                  = "UPDATE jobs SET nextExecutionTime = $1, running = false WHERE id = $2"
	ResetState                = "UPDATE jobs SET nextExecutionTime = NULL, running = false"
	FindNullNextExecutionTime = "SELECT id, name, crontabString, command, arguments, timeout FROM jobs WHERE nextExecutionTime IS NULL LIMIT 100"
	SetNextExecutionTime      = "UPDATE jobs SET nextExecutionTime = $1 WHERE id = $2"
)
