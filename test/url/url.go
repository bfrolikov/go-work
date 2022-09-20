package url

import (
	"fmt"
	"go-work/internal/model"
	"os"
)

func CreateJob() string {
	return fmt.Sprintf("http://localhost:%s/api/v1/job/", os.Getenv("TEST_SERVER_PORT"))
}
func GetJobById(id model.JobId) string {
	return fmt.Sprintf("http://localhost:%s/api/v1/job/%d/", os.Getenv("TEST_SERVER_PORT"), id)
}

func GetJobByName(name string) string {
	return fmt.Sprintf("http://localhost:%s/api/v1/job/%s/", os.Getenv("TEST_SERVER_PORT"), name)
}

func DeleteJob(id model.JobId) string {
	return fmt.Sprintf("http://localhost:%s/api/v1/job/%d/", os.Getenv("TEST_SERVER_PORT"), id)
}
