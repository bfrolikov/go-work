package data

import "time"

type JobData struct {
	Name          string        `json:"name"`
	CrontabString string        `json:"crontabString"`
	ScriptPath    string        `json:"scriptPath"`
	Timeout       time.Duration `json:"timeout"`
}

var InitialJobs = []JobData{
	{"Run every minute",
		"*/1 * * * *",
		"./test_job1.sh",
		250000000,
	},
	{
		"Run every 2 minutes",
		"*/2 * * * *",
		"./test_job2.sh",
		250000000,
	},
}

var CreatedJob = JobData{
	"Newly created",
	"1 2 3 4 5",
	"./test_job3.sh",
	250000000,
}
