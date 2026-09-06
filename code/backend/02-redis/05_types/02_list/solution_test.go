package listx

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func connect(t *testing.T) *redis.Client {
	t.Helper()
	c := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	if err := c.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("连接 Redis 失败（确认已 docker compose up -d redis）: %v", err)
	}
	if err := c.FlushDB(context.Background()).Err(); err != nil {
		t.Fatalf("FlushDB 失败: %v", err)
	}
	return c
}

func TestTaskQueue(t *testing.T) {
	c := connect(t)
	defer c.Close()
	ctx := context.Background()

	if err := SubmitTask(ctx, c, "send-email"); err != nil {
		t.Fatalf("SubmitTask 失败: %v", err)
	}
	task, err := FetchTask(ctx, c, 2*time.Second)
	if err != nil || task != "send-email" {
		t.Fatalf("应取到 send-email，实际 %q, err=%v", task, err)
	}
	// 队列已空，应超时返回 redis.Nil
	if _, err := FetchTask(ctx, c, 500*time.Millisecond); !errors.Is(err, redis.Nil) {
		t.Fatalf("空队列应返回 redis.Nil，实际 err=%v", err)
	}
}

func TestPushFeed(t *testing.T) {
	c := connect(t)
	defer c.Close()
	ctx := context.Background()

	// 依次发 7 条动态，列表应只留最新 5 条
	for _, p := range []string{"p1", "p2", "p3", "p4", "p5", "p6", "p7"} {
		if _, err := PushFeed(ctx, c, 1001, p); err != nil {
			t.Fatalf("PushFeed(%s) 失败: %v", p, err)
		}
	}
	feed, err := PushFeed(ctx, c, 1001, "p8")
	if err != nil {
		t.Fatalf("PushFeed 失败: %v", err)
	}
	want := []string{"p8", "p7", "p6", "p5", "p4"}
	if len(feed) != len(want) {
		t.Fatalf("feed 长度应为 5，实际 %d: %v", len(feed), feed)
	}
	for i := range want {
		if feed[i] != want[i] {
			t.Fatalf("feed[%d] 应为 %s，实际 %s（完整 %v）", i, want[i], feed[i], feed)
		}
	}
}
