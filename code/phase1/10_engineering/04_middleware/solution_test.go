package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
}

func panicHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("something went wrong")
	})
}

func TestLogger(t *testing.T) {
	var logs []string
	logFn := func(msg string) { logs = append(logs, msg) }

	handler := Logger(logFn)(okHandler())

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("期望200，得到 %d", rec.Code)
	}
	if len(logs) == 0 {
		t.Fatal("应有日志输出")
	}
	if !strings.Contains(logs[0], "GET") || !strings.Contains(logs[0], "/test") {
		t.Errorf("日志应包含方法和路径: %q", logs[0])
	}
}

func TestRecovery(t *testing.T) {
	handler := Recovery()(panicHandler())

	req := httptest.NewRequest("GET", "/panic", nil)
	rec := httptest.NewRecorder()

	// 不应 panic
	handler.ServeHTTP(rec, req)

	if rec.Code != 500 {
		t.Errorf("panic 后应返回500，得到 %d", rec.Code)
	}
}

func TestAuth_ValidToken(t *testing.T) {
	handler := Auth("secret-token")(okHandler())

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("合法 token 应通过，得到 %d", rec.Code)
	}
}

func TestAuth_InvalidToken(t *testing.T) {
	handler := Auth("secret-token")(okHandler())

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != 401 {
		t.Errorf("非法 token 应返回401，得到 %d", rec.Code)
	}
}

func TestAuth_NoToken(t *testing.T) {
	handler := Auth("secret")(okHandler())

	req := httptest.NewRequest("GET", "/protected", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != 401 {
		t.Errorf("无 token 应返回401，得到 %d", rec.Code)
	}
}

func TestChain(t *testing.T) {
	var order []string

	m1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m1-before")
			next.ServeHTTP(w, r)
			order = append(order, "m1-after")
		})
	}
	m2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m2-before")
			next.ServeHTTP(w, r)
			order = append(order, "m2-after")
		})
	}

	handler := Chain(m1, m2)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	expected := []string{"m1-before", "m2-before", "handler", "m2-after", "m1-after"}
	if len(order) != len(expected) {
		t.Fatalf("执行顺序错误：%v", order)
	}
	for i, v := range order {
		if v != expected[i] {
			t.Errorf("顺序[%d] = %q，期望 %q。完整: %v", i, v, expected[i], order)
			break
		}
	}
}

func TestChain_WithRecoveryAndLogger(t *testing.T) {
	var logs []string
	logFn := func(msg string) { logs = append(logs, msg) }

	handler := Chain(Logger(logFn), Recovery())(panicHandler())

	req := httptest.NewRequest("GET", "/oops", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != 500 {
		t.Errorf("应返回500，得到 %d", rec.Code)
	}
	if len(logs) == 0 {
		t.Error("Logger 应有输出")
	}
}
