# HTTP 中间件链

## 难度：⭐⭐ 中等

## 考点
- 函数作为一等公民
- 装饰器模式
- HTTP Handler 组合
- panic recovery

## 提示
1. Chain 从后往前包装：最后一个中间件最先接触 handler
2. Logger：在调用 next 前记录日志
3. Recovery：defer + recover 捕获 panic
4. Auth：检查 "Bearer xxx" 格式的 token
