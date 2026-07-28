//go:build ignore

package answer

import "time"

type RateLimiter struct {
	tokens chan struct{}
	ticker *time.Ticker
	stop   chan struct{}
}

func NewRateLimiter(rate int, burst int) *RateLimiter {
	r := &RateLimiter{
		tokens: make(chan struct{}, burst),
		ticker: time.NewTicker(time.Second / time.Duration(rate)),
		stop:   make(chan struct{}),
	}

	go func() {
		for {
			select {
			case <-r.ticker.C:
				select {
				case r.tokens <- struct{}{}:
				default: // 桶满了，丢弃
				}
			case <-r.stop:
				return
			}
		}
	}()

	return r
}

func (r *RateLimiter) Allow() bool {
	select {
	case <-r.tokens:
		return true
	default:
		return false
	}
}

func (r *RateLimiter) Wait() {
	<-r.tokens
}

func (r *RateLimiter) Stop() {
	r.ticker.Stop()
	close(r.stop)
}
