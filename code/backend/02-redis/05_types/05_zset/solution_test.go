package zsetx

import (
	"context"
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

func TestHotRank(t *testing.T) {
	c := connect(t)
	defer c.Close()
	ctx := context.Background()

	AddHot(ctx, c, "Redis", 10)
	AddHot(ctx, c, "Go", 20)
	AddHot(ctx, c, "Redis", 5)
	AddHot(ctx, c, "Kafka", 8)

	top, err := TopN(ctx, c, 2)
	if err != nil {
		t.Fatalf("TopN 失败: %v", err)
	}
	if len(top) != 2 {
		t.Fatalf("应返回 2 条，实际 %d", len(top))
	}
	if top[0].Member != "Go" || top[0].Score != 20 {
		t.Errorf("第 1 名应为 Go(20)，实际 %v(%v)", top[0].Member, top[0].Score)
	}
	if top[1].Member != "Redis" || top[1].Score != 15 {
		t.Errorf("第 2 名应为 Redis(15)，实际 %v(%v)", top[1].Member, top[1].Score)
	}
}

func TestDelayQueue(t *testing.T) {
	c := connect(t)
	defer c.Close()
	ctx := context.Background()
	now := time.Now()

	AddDelayTask(ctx, c, "expired-coupon", now.Add(-time.Second)) // 已到期
	AddDelayTask(ctx, c, "future-report", now.Add(time.Hour))     // 一小时后

	due, err := PopDueTasks(ctx, c, now)
	if err != nil {
		t.Fatalf("PopDueTasks 失败: %v", err)
	}
	if len(due) != 1 || due[0] != "expired-coupon" {
		t.Fatalf("到期任务应只有 [expired-coupon]，实际 %v", due)
	}
	// 再取应为空（已删除），未到期任务不能被取走
	due, err = PopDueTasks(ctx, c, now)
	if err != nil || len(due) != 0 {
		t.Fatalf("再次取出应为空，实际 %v, err=%v", due, err)
	}
}
