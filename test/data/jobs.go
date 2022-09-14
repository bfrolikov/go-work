package data

import (
	"go-work/internal/model"
	"time"
)

type JobData struct {
	Name          string        `json:"name"`
	CrontabString string        `json:"crontabString"`
	ScriptPath    string        `json:"scriptPath"`
	Timeout       time.Duration `json:"timeout"`
}

var InitialJobs = []model.Job{
	{0,
		"Run every minute",
		"*/1 * * * *",
		"./test_job1.sh",
		250000000,
	},
	{
		0,
		"Run every 2 minutes",
		"*/2 * * * *",
		"./test_job2.sh",
		250000000,
	},
}
