package data

import (
	"go-work/internal/model"
)

type JobRequestData struct {
	Name          string   `json:"name"`
	CrontabString string   `json:"crontabString"`
	Command       string   `json:"command"`
	Arguments     []string `json:"arguments"`
	Timeout       uint     `json:"timeout"`
}

var InitialJobs = []model.Job{
	{0,
		"run_every_minute1",
		"*/1 * * * *",
		"python",
		[]string{"test_job1.py"},
		15,
	},
	{
		0,
		"run_every_2_minutes",
		"*/2 * * * *",
		"python",
		[]string{"test_job2.py"},
		15,
	},
}

var JobIntervals = []uint{1, 2}