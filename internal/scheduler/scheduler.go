package scheduler

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

func New(storage model.JobStorage, pingInterval time.Duration) *Scheduler {
	skd := Scheduler{storage, pingInterval, make(chan model.Job), &sync.WaitGroup{}}
	skd.stopWg.Add(2)
	return &skd
}

func (skd *Scheduler) Start(ctx context.Context) {
	go skd.startDueJobs(ctx)
	go skd.monitorDone(ctx)
	skd.stopWg.Wait()
}

func (skd *Scheduler) startDueJobs(ctx context.Context) {
	defer skd.stopWg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(skd.pingInterval):
			jobs, err := skd.storage.MarkDueJobsRunning(ctx)
			if err != nil {
				log.WithFields(log.Fields{
					"error": err,
				}).Error("Error marking due jobs running")
			}

			for _, job := range jobs {
				timeoutCtx, cancel := context.WithTimeout(ctx, job.Timeout)
				log.WithFields(log.Fields{
					"job": job,
				}).Info("Executing job")
				err = exec.CommandContext(timeoutCtx, job.Command).Run()
				cancel()
				if err != nil {
					log.WithFields(log.Fields{
						"error": err,
						"job":   job,
					}).Error("Error executing job") //FIXME: wrap errors
				}

				err = skd.storage.MarkJobDone(ctx, job)
				if err != nil {
					log.WithFields(log.Fields{
						"error": err,
						"job":   job,
					}).Error("Error marking job done")
				}
			}
		}
	}
}

func (skd *Scheduler) monitorDone(ctx context.Context) {
	defer skd.stopWg.Done()
	for {
		select {
		case job := <-skd.doneChannel:
			err := skd.storage.MarkJobDone(ctx, job)
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
