// Module 11, example 1: data races and the sync toolbox.
//
// Run this twice:
//
//	go run 01_races_and_sync.go          # "works", but the racy total is wrong/unstable
//	go run -race 01_races_and_sync.go    # the race detector names the guilty lines
//
// The program shows the SAME problem (1000 goroutines incrementing a counter)
// solved four ways: racy (broken), mutex, atomic, and an RWMutex-protected map.
package main

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// ---------------------------------------------------------------------------
// A map protected by an RWMutex. Plain maps are NOT safe for concurrent use:
// a concurrent write can corrupt the map and crash the program ("fatal error:
// concurrent map writes"). Wrapping it in a small struct keeps the locking
// discipline in ONE place instead of sprinkled around the codebase.
// ---------------------------------------------------------------------------
type SafeMap struct {
	mu sync.RWMutex
	m  map[string]int
}

func NewSafeMap() *SafeMap {
	return &SafeMap{m: make(map[string]int)}
}

// Inc uses the full write lock: it mutates the map.
func (s *SafeMap) Inc(key string) {
	s.mu.Lock()
	defer s.mu.Unlock() // defer right after Lock: early returns can't leak the lock
	s.m[key]++
}

// Get uses RLock: many readers may run concurrently, which is the whole
// point of RWMutex for read-heavy workloads.
func (s *SafeMap) Get(key string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.m[key]
}

func main() {
	const n = 1000
	var wg sync.WaitGroup

	// ------------------------------------------------------------------
	// 1) THE BUG: unsynchronized counter. counter++ is read+add+write,
	//    so goroutines overwrite each other's updates. With -race the
	//    detector prints a "WARNING: DATA RACE" report pointing here.
	// ------------------------------------------------------------------
	racy := 0
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			racy++ // DATA RACE (intentional, for teaching)
		}()
	}
	wg.Wait()
	fmt.Printf("racy counter:    %4d (want %d — often less, result is undefined!)\n", racy, n)

	// ------------------------------------------------------------------
	// 2) FIX A: sync.Mutex. Only one goroutine at a time may hold the
	//    lock, so read+add+write happens as one indivisible step.
	// ------------------------------------------------------------------
	var mu sync.Mutex
	locked := 0
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mu.Lock()
			locked++
			mu.Unlock()
		}()
	}
	wg.Wait()
	fmt.Printf("mutex counter:   %4d (always exact)\n", locked)

	// ------------------------------------------------------------------
	// 3) FIX B: sync/atomic. For a single integer, the typed atomics are
	//    lock-free and cheap. Use atomics for ONE value; the moment two
	//    fields must stay consistent with each other, use a mutex.
	// ------------------------------------------------------------------
	var atomicCounter atomic.Int64
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			atomicCounter.Add(1)
		}()
	}
	wg.Wait()
	fmt.Printf("atomic counter:  %4d (always exact)\n", atomicCounter.Load())

	// ------------------------------------------------------------------
	// 4) FIX C: the SafeMap. 4 goroutines × 250 increments spread over
	//    two keys, plus concurrent readers — all safe.
	// ------------------------------------------------------------------
	sm := NewSafeMap()
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 250; i++ {
				if w%2 == 0 {
					sm.Inc("even-workers")
				} else {
					sm.Inc("odd-workers")
				}
				_ = sm.Get("even-workers") // concurrent reads are fine under RLock
			}
		}(w)
	}
	wg.Wait()
	fmt.Printf("safe map:        even=%d odd=%d (total %d)\n",
		sm.Get("even-workers"), sm.Get("odd-workers"),
		sm.Get("even-workers")+sm.Get("odd-workers"))

	// ------------------------------------------------------------------
	// 5) sync.Once: expensive initialization that must happen exactly
	//    once, even when many goroutines race to trigger it. Callers
	//    that lose the race BLOCK until the winner's Do() returns, so
	//    everyone observes fully-initialized state.
	// ------------------------------------------------------------------
	var (
		once     sync.Once
		initRuns atomic.Int32
		config   string
	)
	loadConfig := func() {
		initRuns.Add(1) // count how many times this actually executes
		config = "config loaded from disk"
	}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			once.Do(loadConfig) // 10 callers, 1 execution
		}()
	}
	wg.Wait()
	fmt.Printf("sync.Once:       %q, loader ran %d time(s) despite 10 callers\n",
		config, initRuns.Load())
}
