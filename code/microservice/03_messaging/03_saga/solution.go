package saga

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Step 是 saga 的一步。
type Step struct {
	Name string
	// Do 执行正向操作（如"扣库存""创建订单"）。
	Do func(ctx context.Context) error
	// Compensate 补偿（如"返还库存""取消订单"），必须幂等。
	Compensate func(ctx context.Context) error
}

// Config 编排参数。
type Config struct {
	// CompensateAttempts 单步补偿最多尝试几次。
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

// Run 执行 saga：
//   - 全部成功 → nil；
//   - 第 i 步失败 → 逆序补偿 0..i-1，返回 *SagaError；
//   - ctx 取消 → 停止正向执行，并对已完成步骤补偿。
func (s *Saga) Run(ctx context.Context) error {
	// TODO: 实现你的代码
	panic("not implemented")
}

// SagaError 记录失败步骤与补偿结果。
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

// 常见业务错误，测试用。
var (
	ErrOutOfStock     = errors.New("out of stock")
	ErrPaymentDecline = errors.New("payment declined")
)
