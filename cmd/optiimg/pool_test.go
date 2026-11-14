package main

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDefaultWorkers(t *testing.T) {
	for in, want := range map[int]int{1: 1, 4: 2, 8: 4, 10: 4, 32: 4} {
		if got := DefaultWorkers(in); got != want {
			t.Errorf("DefaultWorkers(%d)=%d want %d", in, got, want)
		}
	}
}

func TestTokensFor(t *testing.T) {
	cases := []struct {
		px    int64
		total int
		want  int
	}{{12e6, 4, 1}, {20e6, 4, 2}, {20e6, 1, 1}, {50e6, 4, 4}}
	for _, c := range cases {
		if got := TokensFor(c.px, c.total); got != c.want {
			t.Errorf("TokensFor(%d,%d)=%d want %d", c.px, c.total, got, c.want)
		}
	}
}

func items(n int) []Item {
	out := make([]Item, n)
	for i := range out {
		out[i] = Item{Path: string(rune('a' + i))}
	}
	return out
}

func TestRunConcurrencyCap(t *testing.T) {
	var cur, max int32
	job := func(ctx context.Context, it Item, acquire func(int64) func()) Result {
		release := acquire(1000)
		defer release()
		c := atomic.AddInt32(&cur, 1)
		for {
			m := atomic.LoadInt32(&max)
			if c <= m || atomic.CompareAndSwapInt32(&max, m, c) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		atomic.AddInt32(&cur, -1)
		return Result{Item: it}
	}
	n := 0
	for range Run(context.Background(), items(20), 3, job) {
		n++
	}
	if n != 20 || max != 3 {
		t.Errorf("results %d, max concurrency %d", n, max)
	}
}

func TestRunBigImageAlone(t *testing.T) {
	var running, bigRunning int32
	var violated int32
	its := items(6)
	job := func(ctx context.Context, it Item, acquire func(int64) func()) Result {
		px := int64(1000)
		if it.Path == "a" {
			px = 50e6
		}
		release := acquire(px)
		defer release()
		n := atomic.AddInt32(&running, 1)
		if it.Path == "a" {
			atomic.StoreInt32(&bigRunning, 1)
			if n != 1 {
				atomic.StoreInt32(&violated, 1)
			}
		} else if atomic.LoadInt32(&bigRunning) == 1 {
			atomic.StoreInt32(&violated, 1)
		}
		time.Sleep(30 * time.Millisecond)
		if it.Path == "a" {
			atomic.StoreInt32(&bigRunning, 0)
		}
		atomic.AddInt32(&running, -1)
		return Result{Item: it}
	}
	for range Run(context.Background(), its, 4, job) {
	}
	if violated != 0 {
		t.Error("a job ran alongside the big image")
	}
}

func TestRunCancelStopsDispatch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var mu sync.Mutex
	cancelled := false
	var startedAfter int32
	job := func(ctx context.Context, it Item, acquire func(int64) func()) Result {
		mu.Lock()
		if cancelled {
			startedAfter++
		}
		mu.Unlock()
		time.Sleep(20 * time.Millisecond)
		return Result{Item: it}
	}
	n := 0
	for range Run(ctx, items(20), 2, job) {
		n++
		if n == 1 {
			mu.Lock()
			cancel()
			cancelled = true
			mu.Unlock()
		}
	}
	if n >= 20 {
		t.Errorf("got all %d results despite cancel", n)
	}
	// A worker that checked ctx just before cancel() may still start its job,
	// so up to workers-1 stragglers are legitimate; dispatch itself must stop.
	if startedAfter > 1 {
		t.Errorf("%d jobs started after cancel", startedAfter)
	}
}
