# 重试包装器

## 难度：⭐⭐ 中等

## 考点
- 指数退避（Exponential Backoff）
- 泛型重试函数
- 接口断言判断可重试性

## 提示
1. 循环 maxRetries+1 次（含首次执行）
2. 每次失败后 sleep，时间按 multiplier 增长，不超过 maxWait
3. ShouldRetry 用类型断言 `err.(Retryable)` 判断
