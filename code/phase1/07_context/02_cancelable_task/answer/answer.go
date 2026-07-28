//go:build ignore

package answer

import (
	"context"
	"time"
)

func LongTask(ctx context.Context, steps int, stepDuration time.Duration) (int, error) {
	for i := 0; i < steps; i++ {
		select {
		case <-ctx.Done():
			return i, ctx.Err()
		case <-time.After(stepDuration):
		}
	}
	return steps, nil
}

func TaskWithCleanup(ctx context.Context, work func(ctx context.Context) error, cleanup func()) error {
	defer cleanup()
	return work(ctx)
}

func Heartbeat(ctx context.Context, interval time.Duration) <-chan time.Time {
	ch := make(chan time.Time)
	go func() {
		defer close(ch)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case t := <-ticker.C:
				select {
				case ch <- t:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch
}
