package sheduler

import (
	"context"
	log "github.com/sirupsen/logrus"
	"go-work/internal/model"
	"os/exec"
	"sync"
	"time"
)

type Scheduler struct {
	storage      model.JobStorage
	pingInterval time.Duration
	doneChannel  chan model.Job
	stopWg       *sync.WaitGroup
}

func New(storage model.JobStorage, pingInterval time.Duration) Scheduler {
	sched := Scheduler{storage, pingInterval, make(chan model.Job), &sync.WaitGroup{}}
	sched.stopWg.Add(2)
	return sched
}

func (scheduler Scheduler) Start(ctx context.Context) {
	go scheduler.startDueJobs(ctx)
	go scheduler.monitorDone(ctx)
	scheduler.stopWg.Wait()
}

func (scheduler Scheduler) startDueJobs(ctx context.Context) {
	defer scheduler.stopWg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		default:
			jobs, err := scheduler.storage.MarkDueJobsRunning(ctx)
			if err != nil {
				log.WithFields(log.Fields{
					"error": err,
				}).Error("Error marking due jobs running")
			}

			for _, job := range jobs {
				timeoutCtx, cancel := context.WithTimeout(ctx, job.Timeout)
				err = exec.CommandContext(timeoutCtx, job.ScriptPath).Run()
				cancel()
				if err != nil {
					log.WithFields(log.Fields{
						"error": err,
						"job":   job,
					}).Error("Error executing job")
				}
			}
			time.Sleep(scheduler.pingInterval)
		}
	}
}

func (scheduler Scheduler) monitorDone(ctx context.Context) {
	defer scheduler.stopWg.Done()
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
