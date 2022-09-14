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
		"run_every_minute",
		"*/1 * * * *",
		"test_job1.py",
		250000000,
	},
	{
		0,
		"run_every_2_minutes",
		"*/2 * * * *",
		"test_job2.py",
		250000000,
	},
}
