package main

import (
	"context"
	"fmt"
	"github.com/jessevdk/go-flags"
	_ "github.com/lib/pq"
	log "github.com/sirupsen/logrus"
	"go-work/internal/http"
	"go-work/internal/model"
	"go-work/internal/scheduler"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type Options struct {
	ServerPort uint   `long:"server-port" description:"Port for server to listen on" default:"8080"`
	DbHost     string `long:"db-url" description:"Database host url" required:"true"`
	DbPort     uint   `long:"db-port" description:"Database port" default:"5432"`
	DbUser     string `long:"db-login" description:"Database user login" required:"true"`
	DbName     string `long:"db-name" description:"Database name" required:"true"`
	Intervals  []uint `long:"interval" description:"Query intervals for schedulers" required:"true"`
}

const serverShutdownTimeout = 30 * time.Second

func main() {
	opts := Options{}
	_, err := flags.Parse(&opts)
	if err != nil {
		log.Fatal(fmt.Errorf("could not parse command line args: %w", err))
	}
	dataSourceName := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		opts.DbHost,
		opts.DbPort,
		opts.DbUser,
		os.Getenv("POSTGRES_PASSWORD"),
		opts.DbName,
	)
	background := context.Background()
	var storage model.JobStorage
	storage, err = model.NewSQLJobStorage(background, "postgres", dataSourceName)
	if err != nil {
		log.Fatal(fmt.Errorf("could not create job storage: %w", err))
	}
	server, err := http.NewJobServer(storage, fmt.Sprintf(":%d", opts.ServerPort))
	if err != nil {
		log.Fatal(fmt.Errorf("could not create job server: %w", err))
	}
	cancelCtx, cancel := context.WithCancel(background)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	wg := sync.WaitGroup{}
	wg.Add(len(opts.Intervals) + 1)
	for _, interval := range opts.Intervals {
		go func(interval uint) {
			defer wg.Done()
			scheduler.New(storage, time.Duration(interval)*time.Second).Start(cancelCtx)
		}(interval)
	}
	go func() {
		defer wg.Done()
		if err := server.ListenAndServe(); err != nil {
			log.Error(fmt.Errorf("listen and serve error: %w", err))
		}
	}()
	<-sigs
	cancel()
	timeoutCtx, timeoutCancel := context.WithTimeout(background, serverShutdownTimeout)
	defer timeoutCancel()
	if err = server.Shutdown(timeoutCtx); err != nil {
		log.Error(fmt.Errorf("failed to shutdown server: %w", err))
	}
	wg.Wait()
}
