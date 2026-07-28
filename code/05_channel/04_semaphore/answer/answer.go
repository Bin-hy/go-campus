//go:build ignore

package answer

import "time"

type Semaphore struct {
	ch chan struct{}
}

func NewSemaphore(n int) *Semaphore {
	return &Semaphore{ch: make(chan struct{}, n)}
}

func (s *Semaphore) Acquire() {
	s.ch <- struct{}{}
}

func (s *Semaphore) TryAcquire(timeout time.Duration) bool {
	select {
	case s.ch <- struct{}{}:
		return true
	case <-time.After(timeout):
		return false
	}
}

func (s *Semaphore) Release() {
	<-s.ch
}

func (s *Semaphore) Available() int {
	return cap(s.ch) - len(s.ch)
}
