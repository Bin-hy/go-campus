package encoding

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
	return c
}

func TestEncodingSwitch(t *testing.T) {
	c := connect(t)
	defer c.Close()

	before, after, err := EncodingSwitch(context.Background(), c)
	if err != nil {
		t.Fatalf("EncodingSwitch 失败: %v", err)
	}

	if before != "intset" {
		t.Errorf("纯整数 set 编码应为 intset，实际 %s", before)
	}
	if after != "listpack" && after != "hashtable" {
		t.Errorf("混入字符串后编码应为 listpack/hashtable，实际 %s", after)
	}
	t.Logf("编码切换验证通过：%s → %s", before, after)
}
