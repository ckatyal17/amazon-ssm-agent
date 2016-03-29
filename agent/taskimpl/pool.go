// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Amazon Software License (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/asl/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package taskimpl

import (
	"fmt"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/agent/times"
)

// pool implements a task pool where all jobs are managed by a root task
type pool struct {
	log        log.T
	jobQueue   chan JobToken
	nWorkers   int
	doneWorker chan struct{}
	isShutdown bool
	clock      times.Clock
	mut        sync.Mutex
	rootTask   *Task
}

// JobToken embeds a job and its associated info
type JobToken struct {
	id         string
	job        task.Job
	cancelFlag *CancelFlag
	log        log.T
}

// NewPool creates a new task pool and launches maxParallel workers.
// The cancelWaitDuration parameter defines how long to wait for a job
// to complete a cancellation request.
func NewPool(log log.T, maxParallel int, cancelWaitDuration time.Duration, clock times.Clock) task.Pool {
	p := &pool{
		log:        log,
		jobQueue:   make(chan JobToken),
		nWorkers:   maxParallel,
		doneWorker: make(chan struct{}),
		clock:      clock,
	}

	p.rootTask = NewTask()

	// defines the job processing function.
	processor := func(j JobToken) {
		defer p.rootTask.DeleteJob(j.id)
		process(j.log, j.job, j.cancelFlag, cancelWaitDuration, p.clock)
	}

	// start the workers
	p.start(processor)

	return p
}

// Shutdown cancels all the jobs in this pool and shuts down the workers.
func (p *pool) Shutdown() {
	// ShutDown and delete all jobs
	p.ShutDownAll()

	p.mut.Lock()
	defer p.mut.Unlock()
	if !p.isShutdown {
		// close the channel to makes all workers terminate once the pending
		// jobs have been consumed (the pending jobs are in the Canceled state
		// so they will simply be discarded)
		close(p.jobQueue)
		p.isShutdown = true
	}
}

// ShutdownAndWait calls Shutdown then waits until all the workers have exited
// or until the timeout has elapsed, whichever comes first. Returns true if all
// workers terminated before the timeout or false if the timeout expired.
func (p *pool) ShutdownAndWait(timeout time.Duration) (finished bool) {
	p.Shutdown()

	timeoutTimer := p.clock.After(timeout)
	workersRunning := p.nWorkers
	for workersRunning > 0 {
		select {
		case <-p.doneWorker:
			workersRunning--
			if workersRunning == 0 {
				p.log.Debug("Pool shutdown normally.")
				return true
			}
			p.log.Debugf("Pool worker done; %d still running", workersRunning)

		case <-timeoutTimer:
			p.log.Debugf("Pool shutdown timed out with %d workers still running", workersRunning)
			return false
		}
	}
	return true
}

// start starts the workers of this pool
func (p *pool) start(jobProcessor func(JobToken)) {
	for i := 0; i < p.nWorkers; i++ {
		workerName := fmt.Sprintf("worker-%d", i)
		go func() {
			defer p.workerDone()
			worker(workerName, p.jobQueue, jobProcessor)
		}()
	}
}

// workerDone signals that a worker has terminated.
func (p *pool) workerDone() {
	p.doneWorker <- struct{}{}
}

// worker processes jobs from a channel.
func worker(workerName string, queue chan JobToken, processor func(JobToken)) {
	for token := range queue {
		if !token.cancelFlag.Canceled() {
			processor(token)
		}
	}
}

// Submit adds a job to the execution queue of this pool.
func (p *pool) Submit(log log.T, jobID string, job task.Job) (err error) {
	token := JobToken{
		id:         jobID,
		job:        job,
		cancelFlag: NewCancelFlag(),
		log:        log,
	}
	err = p.rootTask.AddJob(jobID, &token)
	if err != nil {
		return
	}
	p.jobQueue <- token
	return
}

// Cancel cancels the job with the given id.
func (p *pool) Cancel(jobID string) (canceled bool) {
	jobToken, found := p.rootTask.GetJob(jobID)
	if !found {
		return false
	}

	jobToken.cancelFlag.Set(task.Canceled)
	return true
}

// Cancel cancels and deletes all the jobs of this task.
func (p *pool) CancelAll() {
	// remove jobs from task and save them to a local variable
	jobs := p.rootTask.DeleteAllJobs()

	// cancel each job
	for _, token := range jobs {
		token.cancelFlag.Set(task.Canceled)
	}
}

// Cancel cancels and deletes all the jobs of this task.
func (p *pool) ShutDownAll() {
	// remove jobs from task and save them to a local variable
	jobs := p.rootTask.DeleteAllJobs()

	// cancel each job
	for _, token := range jobs {
		token.cancelFlag.Set(task.ShutDown)
	}
}
