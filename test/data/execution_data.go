package data

import (
	"fmt"
	"sync"
	"time"
)

type jobExecutionData struct {
	previousExecutionTime *time.Time
	lastExecutionTime     *time.Time
	interval              uint
}

const IntervalEps = time.Second * 10

type ExecutionData struct {
	data map[uint]*jobExecutionData
	lock *sync.Mutex
}

func NewExecutionData() *ExecutionData {
	executionData := ExecutionData{make(map[uint]*jobExecutionData), &sync.Mutex{}}
	executionData.Reset()
	return &executionData
}

func (ed *ExecutionData) SignalExecution(interval uint) {
	ed.lock.Lock()
	defer ed.lock.Unlock()
	if execData := ed.data[interval]; execData != nil {
		execData.previousExecutionTime = execData.lastExecutionTime
		now := time.Now()
		execData.lastExecutionTime = &now
	}
}

func (ed *ExecutionData) Reset() {
	ed.lock.Lock()
	defer ed.lock.Unlock()
	for _, i := range JobIntervals {
		ed.data[i] = &jobExecutionData{nil, nil, i}
	}
}

func (ed *ExecutionData) MaxInterval() uint {
	ed.lock.Lock()
	defer ed.lock.Unlock()
	var result uint = 0
	for _, v := range ed.data {
		if v.interval > result {
			result = v.interval
		}
	}
	return result
}

func (ed *ExecutionData) ValidateExecutionData() error {
	ed.lock.Lock()
	defer ed.lock.Unlock()

	for _, v := range ed.data {
		if v.previousExecutionTime == nil || v.lastExecutionTime == nil {
			return fmt.Errorf("task with interval %d did not execute twice", v.interval)
		}
		maxDifference := time.Duration(v.interval)*time.Minute + IntervalEps
		actualDifference := v.lastExecutionTime.Sub(*v.previousExecutionTime)
		if actualDifference > maxDifference {
			return fmt.Errorf(
				"time diffecence between executions %s exceeded the maximum of %s for task with interval %dmin",
				actualDifference,
				maxDifference,
				v.interval,
			)
		}
	}
	return nil
}
