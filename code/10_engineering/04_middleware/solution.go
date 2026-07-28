package middleware

import "net/http"

// Middleware 中间件类型：接收 handler 返回新 handler
type Middleware func(http.Handler) http.Handler

// Chain 将多个中间件组合成一个
// 执行顺序：第一个中间件最外层（最先执行前置逻辑，最后执行后置逻辑）
func Chain(middlewares ...Middleware) Middleware {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Logger 日志中间件：记录请求方法和路径到 logFn
func Logger(logFn func(string)) Middleware {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Recovery panic 恢复中间件：捕获 panic 返回 500
func Recovery() Middleware {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Auth 认证中间件：检查 Authorization header
// token 匹配时放行，否则返回 401
func Auth(validToken string) Middleware {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}
