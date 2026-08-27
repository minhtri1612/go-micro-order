package worker

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/minhtri1612/go-micro-order/model"
)

// Job represents a task to be processed
type Job struct {
	Order model.Order
}

// Result represents the outcome of job processing
type Result struct {
	OrderID int
	Error   error
}

// Pool represents a worker pool
type Pool struct {
	numWorkers  int
	jobQueue    chan Job
	resultQueue chan Result
	done        chan bool
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewPool creates a new worker pool
func NewPool(numWorkers int, queueSize int) *Pool {
	ctx, cancel := context.WithCancel(context.Background())
	return &Pool{
		numWorkers:  numWorkers,
		jobQueue:    make(chan Job, queueSize),
		resultQueue: make(chan Result, queueSize),
		done:        make(chan bool),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start initializes the worker pool
func (p *Pool) Start(processFunc func(Job) Result) {
	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < p.numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			p.worker(workerID, processFunc)
		}(i)
	}

	// Wait for all workers to complete in a separate goroutine
	go func() {
		wg.Wait()
		close(p.resultQueue)
		p.done <- true
	}()
}

// worker processes jobs from the job queue
func (p *Pool) worker(id int, processFunc func(Job) Result) {
	log.Printf("Worker %d starting\n", id)
	for {
		select {
		case job, ok := <-p.jobQueue:
			if !ok {
				log.Printf("Worker %d shutting down\n", id)
				return
			}
			// Process the job and send the result
			result := processFunc(job)
			p.resultQueue <- result

		case <-p.ctx.Done():
			log.Printf("Worker %d cancelled\n", id)
			return
		}
	}
}

// Submit adds a job to the queue
func (p *Pool) Submit(job Job) {
	p.jobQueue <- job
}

// Results returns the channel for receiving results
func (p *Pool) Results() <-chan Result {
	return p.resultQueue
}

// Stop gracefully shuts down the worker pool
func (p *Pool) Stop() {
	p.cancel()
	close(p.jobQueue)
	<-p.done
}

// ProcessBatch handles a batch of orders with timeout
func ProcessBatch(orders []model.Order, numWorkers int, timeout time.Duration) []Result {
	// Create a worker pool with buffer size equal to number of orders
	pool := NewPool(numWorkers, len(orders))

	// Start the pool with the processing function
	pool.Start(func(job Job) Result {
		// Simulate processing time (replace with actual processing)
		time.Sleep(100 * time.Millisecond)
		return Result{
			OrderID: job.Order.ID,
			Error:   nil,
		}
	})

	// Submit all orders to the pool
	go func() {
		for _, order := range orders {
			pool.Submit(Job{Order: order})
		}
	}()

	// Collect results with timeout
	results := make([]Result, 0, len(orders))
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for i := 0; i < len(orders); i++ {
		select {
		case result := <-pool.Results():
			results = append(results, result)
		case <-timer.C:
			log.Printf("Batch processing timeout after %v\n", timeout)
			pool.Stop()
			return results
		}
	}

	pool.Stop()
	return results
}
