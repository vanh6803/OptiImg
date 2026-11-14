package main

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/semaphore"

	"optiimg/core"
)

// DefaultWorkers returns clamp(numCPU/2, 1, 4).
func DefaultWorkers(numCPU int) int {
	w := numCPU / 2
	if w < 1 {
		w = 1
	}
	if w > 4 {
		w = 4
	}
	return w
}

// TokensFor returns the memory tokens an image of the given size needs.
func TokensFor(pixels int64, total int) int {
	switch {
	case pixels <= 12_000_000:
		return 1
	case pixels <= 40_000_000:
		if total < 2 {
			return total
		}
		return 2
	}
	return total
}

// Result is the outcome of one job.
type Result struct {
	Item     Item
	Report   core.Report
	Err      error
	Duration time.Duration
}

// Job processes one item. acquire blocks until enough memory tokens are free
// and returns the release function.
type Job func(ctx context.Context, it Item, acquire func(pixels int64) (release func())) Result

// Run feeds items to a bounded pool. Cancelling ctx stops starting new work;
// jobs already running finish. The result channel closes when all is done.
func Run(ctx context.Context, items []Item, workers int, job Job) <-chan Result {
	if workers < 1 {
		workers = 1
	}
	sem := semaphore.NewWeighted(int64(workers))
	acquire := func(pixels int64) func() {
		n := int64(TokensFor(pixels, workers))
		_ = sem.Acquire(context.Background(), n)
		return func() { sem.Release(n) }
	}
	queue := make(chan Item, workers*2)
	results := make(chan Result, workers*2)

	go func() {
		defer close(queue)
		for _, it := range items {
			select {
			case <-ctx.Done():
				return
			case queue <- it:
			}
		}
	}()

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for it := range queue {
				if ctx.Err() != nil {
					continue // drain without starting
				}
				results <- job(ctx, it, acquire)
			}
		}()
	}
	go func() {
		wg.Wait()
		close(results)
	}()
	return results
}
