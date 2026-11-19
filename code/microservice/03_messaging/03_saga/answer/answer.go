package saga

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// 参考答案 / 可运行实现。
//
// Saga = 一串本地事务 + 每一步的补偿动作。
// 任一步失败，就按相反顺序执行已完成步骤的补偿（反向补偿）。
// 注意：补偿必须幂等——它可能因为重试/崩溃恢复被执行多次。

// Step 是 saga 的一步。
type Step struct {
	Name string
	// Do 执行正向操作（如"扣库存""创建订单"）。
	Do func(ctx context.Context) error
	// Compensate 补偿（如"返还库存""取消订单"）。
	// 必须幂等，且不能依赖 Do 的返回值（崩溃恢复时可能拿不到）。
	Compensate func(ctx context.Context) error
}

// Config 编排参数。
type Config struct {
	// CompensateAttempts 单步补偿最多尝试几次（网络抖动时重试）。
	CompensateAttempts int
	// CompensateDelay 补偿重试间隔。
	CompensateDelay time.Duration
}

// Saga 顺序执行步骤。
type Saga struct {
	Steps []Step
	Cfg   Config
}

func New(steps []Step, cfg Config) *Saga {
	if cfg.CompensateAttempts <= 0 {
		cfg.CompensateAttempts = 1
	}
	return &Saga{Steps: steps, Cfg: cfg}
}

// SagaError 记录失败步骤与补偿结果，便于排障与人工介入。
type SagaError struct {
	FailedStep        string
	Cause             error
	Compensated       []string
	CompensationFails map[string]error
}

func (e *SagaError) Error() string {
	return fmt.Sprintf("saga failed at %q: %v (compensated %v, compensation failures %v)",
		e.FailedStep, e.Cause, e.Compensated, e.CompensationFails)
}

func (e *SagaError) Unwrap() error { return e.Cause }

// Run 执行 saga。
//   - 全部成功 → nil；
//   - 第 i 步失败 → 逆序补偿 0..i-1，返回 *SagaError；
//   - ctx 取消 → 停止正向执行，并对已完成步骤补偿（尽力而为）。
func (s *Saga) Run(ctx context.Context) error {
	completed := make([]int, 0, len(s.Steps))
	for i, step := range s.Steps {
		if err := ctx.Err(); err != nil {
			return s.compensate(ctx, completed, fmt.Errorf("canceled before %q: %w", step.Name, err))
		}
		if err := step.Do(ctx); err != nil {
			return s.compensate(ctx, completed, &stepFailure{step: step.Name, err: err})
		}
		completed = append(completed, i)
	}
	return nil
}

type stepFailure struct {
	step string
	err  error
}

func (e *stepFailure) Error() string { return e.step + ": " + e.err.Error() }
func (e *stepFailure) Unwrap() error { return e.err }

func (s *Saga) compensate(ctx context.Context, completed []int, cause error) error {
	se := &SagaError{Cause: cause, CompensationFails: make(map[string]error)}
	if sf, ok := cause.(*stepFailure); ok {
		se.FailedStep = sf.step
	} else {
		se.FailedStep = "precondition"
	}

	// 逆序补偿：后做的先撤销。
	for i := len(completed) - 1; i >= 0; i-- {
		step := s.Steps[completed[i]]
		var err error
		for attempt := 0; attempt < s.Cfg.CompensateAttempts; attempt++ {
			err = step.Compensate(ctx)
			if err == nil {
				break
			}
			if s.Cfg.CompensateDelay > 0 && attempt < s.Cfg.CompensateAttempts-1 {
				t := time.NewTimer(s.Cfg.CompensateDelay)
				select {
				case <-ctx.Done():
					t.Stop()
					attempt = s.Cfg.CompensateAttempts // 不再重试
					err = ctx.Err()
				case <-t.C:
				}
			}
		}
		if err != nil {
			se.CompensationFails[step.Name] = err
			continue // 单步补偿失败不能阻断其他补偿
		}
		se.Compensated = append(se.Compensated, step.Name)
	}
	return se
}

// 常见业务错误，测试用。
var (
	ErrOutOfStock     = errors.New("out of stock")
	ErrPaymentDecline = errors.New("payment declined")
)
