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
				log.Errorf("Error marking due jobs running: %s", err)
			}

			for _, job := range jobs {
				go func(job *model.Job) {
					defer func() {
						if rec := recover(); rec != nil {
							log.Errorf("Panic while executing job: %s", rec)
						}
					}()

					timeoutCtx, cancel := context.WithTimeout(ctx, time.Second*time.Duration(job.Timeout))
					log.WithFields(log.Fields{
						"job": job,
					}).Info("Executing job")
					err = exec.CommandContext(timeoutCtx, job.Command, job.Arguments...).Run()
					cancel()
					if err != nil {
						log.WithFields(log.Fields{
							"job": job,
						}).Errorf("Error executing job: %s", err)
					}
					err = skd.storage.MarkJobDone(ctx, job)
					if err != nil {
						log.WithFields(log.Fields{
							"job": job,
						}).Errorf("Error marking job done: %s", err)
					}
				}(job)
			}
		}
	}
}

func (skd *Scheduler) monitorDone(ctx context.Context) {
	defer skd.stopWg.Done()
	for {
		select {
		case job := <-skd.doneChannel:
			err := skd.storage.MarkJobDone(ctx, &job)
			if err != nil {
				log.WithFields(log.Fields{
					"job": job,
				}).Errorf("Error signaling completion of job: %s", err)
			}
		case <-ctx.Done():
			return
		}
	}
}
