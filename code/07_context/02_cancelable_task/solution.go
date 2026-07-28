package cancelable_task

import (
	"context"
	"time"
)

// LongTask 模拟一个长时间运行的任务
// 任务分 steps 步完成，每步耗时 stepDuration
// context 取消时立即返回已完成的步数和 ctx.Err()
func LongTask(ctx context.Context, steps int, stepDuration time.Duration) (completedSteps int, err error) {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// TaskWithCleanup 执行任务，取消时执行清理函数
// 无论是否取消，cleanup 必须被调用恰好一次
func TaskWithCleanup(ctx context.Context, work func(ctx context.Context) error, cleanup func()) error {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Heartbeat 在任务执行期间定期发送心跳
// 返回一个 channel，每隔 interval 发送一次当前时间戳
// context 取消时关闭 channel
func Heartbeat(ctx context.Context, interval time.Duration) <-chan time.Time {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}
