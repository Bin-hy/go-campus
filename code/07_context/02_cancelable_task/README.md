# 可取消任务

## 难度：⭐⭐ 中等

## 考点
- context.WithCancel 取消传播
- 任务中断与清理
- 心跳模式

## 提示
1. LongTask 每步用 select 检查 ctx.Done()
2. TaskWithCleanup 用 defer 确保 cleanup 执行
3. Heartbeat 后台 goroutine + ticker + select ctx.Done()
