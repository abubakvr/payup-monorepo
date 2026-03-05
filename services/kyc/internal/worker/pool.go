package worker

import (
	"context"
	"log"
	"sync"
)

// Pool runs a fixed number of workers that process jobs from a channel.
type Pool struct {
	jobs        chan Job
	numWorkers  int
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// Job is a unit of work. Run is executed by a worker.
type Job struct {
	Run func(ctx context.Context) error
}

// NewPool creates a pool with numWorkers goroutines and a buffered channel of queueSize. Start() must be called to begin processing.
func NewPool(numWorkers int, queueSize int) *Pool {
	if numWorkers < 1 {
		numWorkers = 3
	}
	if queueSize < 1 {
		queueSize = 100
	}
	return &Pool{
		jobs:       make(chan Job, queueSize),
		numWorkers: numWorkers,
	}
}

// Start begins the worker goroutines. Call Stop() to shut down.
func (p *Pool) Start(ctx context.Context) {
	ctx, p.cancel = context.WithCancel(ctx)
	for i := 0; i < p.numWorkers; i++ {
		p.wg.Add(1)
		go p.worker(ctx, i)
	}
}

func (p *Pool) worker(ctx context.Context, id int) {
	defer p.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-p.jobs:
			if !ok {
				return
			}
			if job.Run != nil {
				if err := job.Run(ctx); err != nil {
					log.Printf("worker %d: job failed: %v", id, err)
				}
			}
		}
	}
}

// Submit enqueues a job. Blocks if queue is full. Returns false if context cancelled or pool closed.
func (p *Pool) Submit(ctx context.Context, job Job) bool {
	select {
	case <-ctx.Done():
		return false
	case p.jobs <- job:
		return true
	}
}

// Stop closes the job channel and waits for workers to finish.
func (p *Pool) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
	close(p.jobs)
	p.wg.Wait()
}
