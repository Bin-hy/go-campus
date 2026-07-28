//go:build ignore

package answer

import "time"

type RetryConfig struct {
	MaxRetries  int
	InitialWait time.Duration
	MaxWait     time.Duration
	Multiplier  float64
}

func DefaultConfig() RetryConfig {
	return RetryConfig{MaxRetries: 3, InitialWait: 100 * time.Millisecond, MaxWait: 5 * time.Second, Multiplier: 2.0}
}

func Retry(fn func() error, config RetryConfig) error {
	var err error
	wait := config.InitialWait

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		err = fn()
		if err == nil {
			return nil
		}
		if attempt < config.MaxRetries {
			time.Sleep(wait)
			wait = time.Duration(float64(wait) * config.Multiplier)
			if wait > config.MaxWait {
				wait = config.MaxWait
			}
		}
	}
	return err
}

func RetryWithResult[T any](fn func() (T, error), config RetryConfig) (T, error) {
	var result T
	var err error
	wait := config.InitialWait

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		result, err = fn()
		if err == nil {
			return result, nil
		}
		if attempt < config.MaxRetries {
			time.Sleep(wait)
			wait = time.Duration(float64(wait) * config.Multiplier)
			if wait > config.MaxWait {
				wait = config.MaxWait
			}
		}
	}
	return result, err
}

type Retryable interface {
	IsRetryable() bool
}

func ShouldRetry(err error) bool {
	if err == nil {
		return false
	}
	if r, ok := err.(Retryable); ok {
		return r.IsRetryable()
	}
	return true
}
