package main

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/semaphore"

	"optiimg/src/core"
	"optiimg/src/core/jpegenc"
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

// DefaultMemBudget is the default total memory (bytes) the pool lets
// concurrent jobs claim.
const DefaultMemBudget int64 = 2 << 30

// EstimateMem returns the estimated peak memory for compressing an image.
// Measured on 4K/8K photos: PNG ≈ 25 B/px, JPEG ≈ 45 B/px (jpegli buffers the
// whole image), and ≈ 18 B/px on the low-memory path used above 16 MP.
func EstimateMem(f core.Format, pixels int64) int64 {
	switch {
	case f == core.FormatPNG:
		return pixels * 25
	case pixels > jpegenc.LowMemPixels:
		return pixels * 18
	}
	return pixels * 45
}

// Result is the outcome of one job.
type Result struct {
	Item     Item
	Report   core.Report
	Err      error
	Duration time.Duration
}

// Job processes one item. acquire blocks until enough of the memory budget is
// free and returns the release function.
type Job func(ctx context.Context, it Item, acquire func(bytes int64) (release func())) Result

// Run feeds items to a bounded pool: at most `workers` jobs at once, and their
// estimated memory together stays within budget. Cancelling ctx stops starting new work;
// jobs already running finish. The result channel closes when all is done.
func Run(ctx context.Context, items []Item, workers int, budget int64, job Job) <-chan Result {
	if workers < 1 {
		workers = 1
	}
	if budget < 1 {
		budget = DefaultMemBudget
	}
	sem := semaphore.NewWeighted(budget)
	acquire := func(n int64) func() {
		if n > budget {
			n = budget // too big to share: runs alone
		}
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
