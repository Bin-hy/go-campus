package streamx

import (
	"context"
	"testing"

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

func TestOrderStream(t *testing.T) {
	c := connect(t)
	defer c.Close()
	ctx := context.Background()

	id1, err := PublishOrderEvent(ctx, c, 1001, "created")
	if err != nil || id1 == "" {
		t.Fatalf("PublishOrderEvent 失败: id=%q, err=%v", id1, err)
	}
	if _, err := PublishOrderEvent(ctx, c, 1001, "paid"); err != nil {
		t.Fatalf("PublishOrderEvent 失败: %v", err)
	}

	if err := EnsureGroup(ctx, c, "g1"); err != nil {
		t.Fatalf("EnsureGroup 失败: %v", err)
	}
	if err := EnsureGroup(ctx, c, "g1"); err != nil { // 幂等：重复创建不报错
		t.Fatalf("EnsureGroup 应幂等，实际报错: %v", err)
	}

	msgID, fields, err := ConsumeOne(ctx, c, "g1", "c1")
	if err != nil {
		t.Fatalf("ConsumeOne 失败: %v", err)
	}
	if msgID != id1 {
		t.Errorf("第一条消息 ID 应为 %s，实际 %s", id1, msgID)
	}
	if fields["type"] != "created" || fields["orderID"] != "1001" {
		t.Errorf("字段不符: %v", fields)
	}

	_, fields, err = ConsumeOne(ctx, c, "g1", "c1")
	if err != nil || fields["type"] != "paid" {
		t.Errorf("第二条应为 paid，实际 %v, err=%v", fields, err)
	}

	// 队列已消费完，阻塞超时后应返回 redis.Nil
	if _, _, err := ConsumeOne(ctx, c, "g1", "c1"); err != redis.Nil {
		t.Errorf("无新消息应返回 redis.Nil，实际 %v", err)
	}

	// 全部 ACK 后 Pending 应为 0
	pending, err := c.XPending(ctx, "order:events", "g1").Result()
	if err != nil {
		t.Fatalf("XPending 失败: %v", err)
	}
	if pending.Count != 0 {
		t.Errorf("ACK 后 Pending 应为 0，实际 %d", pending.Count)
	}
}
