package sync2

import (
	"sync"
	"sync/atomic"
	"time"
)

func NewSemaphore(initialCount int) *Semaphore {
	res := &Semaphore{
		counter: int64(initialCount),
	}
	res.cond.L = &res.lock
	return res
}

type Semaphore struct {
	lock    sync.Mutex
	cond    sync.Cond
	counter int64
}

func (s *Semaphore) Release() {
	s.lock.Lock()
	s.counter += 1
	if s.counter >= 0 {
		s.cond.Signal()
	}
	s.lock.Unlock()
}

func (s *Semaphore) Acquire() {
	s.lock.Lock()
	for s.counter < 1 {
		s.cond.Wait()
	}
	s.counter -= 1
	s.lock.Unlock()
}

func (s *Semaphore) AcquireTimeout(timeout time.Duration) bool {
	done := make(chan bool, 1)
	// Gate used to communicate between the threads and decide what the result
	// is. If the main thread decides, we have timed out, otherwise we succeed.
	decided := new(int32)
	go func() {
		s.Acquire()
		if atomic.SwapInt32(decided, 1) == 0 {
			done <- true
		} else {
			// If we already decided the result, and this thread did not win
			s.Release()
		}
	}()
	select {
	case <-done:
		return true
	case <-time.NewTimer(timeout).C:
		if atomic.SwapInt32(decided, 1) == 1 {
			// The other thread already decided the result
			return true
		}
		return false
	}
}
