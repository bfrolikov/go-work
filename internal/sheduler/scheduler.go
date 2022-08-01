package sheduler

import (
	"context"
	log "github.com/sirupsen/logrus"
	"go-work/internal/model"
	"os/exec"
	"time"
)

type Scheduler struct {
	storage      model.JobStorage
	pingInterval time.Duration
	doneChannel  chan model.Job
}

func NewScheduler(storage model.JobStorage, pingInterval time.Duration) Scheduler {
	return Scheduler{storage, pingInterval, make(chan model.Job)}
}

func (scheduler Scheduler) Start(ctx context.Context) {
	go scheduler.startDueJobs(ctx)
	go scheduler.monitorDone(ctx)
	<-ctx.Done()
}

func (scheduler Scheduler) startDueJobs(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			dueJobs, err := scheduler.storage.FindDueJobs(ctx)
			if err != nil {
				log.WithFields(log.Fields{
					"error": err,
				}).Error("Error acquiring due jobs")
			} else {
				for _, job := range dueJobs {
					err = scheduler.storage.MarkJobRunning(ctx, job)
					if err != nil {
						log.WithFields(log.Fields{
							"error": err,
							"job":   job,
						}).Error("Error signalling job running")
						continue
					}

					timeoutCtx, cancel := context.WithTimeout(ctx, job.Timeout)
					err = exec.CommandContext(timeoutCtx, job.ScriptPath).Run()
					cancel()
					if err != nil {
						log.WithFields(log.Fields{
							"error": err,
							"job":   job,
						}).Error("Error executing job")
					}

					scheduler.doneChannel <- job
				}
			}
			time.Sleep(scheduler.pingInterval)
		}
	}
}

func (scheduler Scheduler) monitorDone(ctx context.Context) {
	for {
		select {
		case job := <-scheduler.doneChannel:
			err := scheduler.storage.MarkJobDone(ctx, job)
			if err != nil {
				log.WithFields(log.Fields{
					"error": err,
					"job":   job,
				}).Error("Error signaling completion of job")
			}
		case <-ctx.Done():
			return
		}
	}
}
