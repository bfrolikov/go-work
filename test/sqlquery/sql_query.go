package sqlquery

const (
	DeleteAll    = "DELETE from jobs"
	CreateNewJob = "INSERT INTO jobs (name, crontabString, scriptPath, timeout, nextExecutionTime) values ($1, $2, $3, $4, $5) RETURNING id"
)
