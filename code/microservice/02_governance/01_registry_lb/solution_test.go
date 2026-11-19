package registry_lb

import (
	"testing"
	"time"
)

func TestRegistryTTL(t *testing.T) {
	now := time.Unix(1000, 0)
	reg := NewRegistry(5 * time.Second)
	reg.SetClock(func() time.Time { return now })

	reg.Register("user", "10.0.0.1:9000")
	reg.Register("user", "10.0.0.2:9000")
	reg.Register("user", "10.0.0.3:9000")
	if got := len(reg.Discover("user")); got != 3 {
		t.Fatalf("应有 3 个实例，实际 %d", got)
	}

	// 2 号实例持续心跳，另外两个不再续租。
	for i := 0; i < 6; i++ {
		now = now.Add(time.Second)
		if err := reg.Heartbeat("user", "10.0.0.2:9000"); err != nil {
			t.Fatal(err)
		}
	}

	got := reg.Discover("user")
	if len(got) != 1 || got[0] != "10.0.0.2:9000" {
		t.Fatalf("过期实例应被剔除，实际 %v", got)
	}
	t.Logf("TTL 剔除 OK: %v", got)
}

func TestDeregisterAndHeartbeat(t *testing.T) {
	reg := NewRegistry(time.Minute)
	reg.Register("user", "a")
	if err := reg.Heartbeat("user", "unknown"); err == nil {
		t.Fatal("未注册实例心跳应报错")
	}
	reg.Deregister("user", "a")
	if got := reg.Discover("user"); len(got) != 0 {
		t.Fatalf("注销后应为空，实际 %v", got)
	}
}

func TestRoundRobinEven(t *testing.T) {
	rr := &RoundRobin{}
	addrs := []string{"a", "b", "c"}
	count := map[string]int{}
	for i := 0; i < 6; i++ {
		a, err := rr.Pick(addrs)
		if err != nil {
			t.Fatal(err)
		}
		count[a]++
	}
	for _, a := range addrs {
		if count[a] != 2 {
			t.Fatalf("轮询应均匀，%s 被选 %d 次", a, count[a])
		}
	}
	if _, err := rr.Pick(nil); err != ErrNoInstance {
		t.Fatalf("空列表应返回 ErrNoInstance，实际 %v", err)
	}
	t.Logf("round robin OK: %v", count)
}

func TestP2CAvoidsLoadedInstance(t *testing.T) {
	p := NewP2C(42)
	addrs := []string{"a", "b", "c"}
	for i := 0; i < 100; i++ {
		p.Begin("a")
	}
	for i := 0; i < 200; i++ {
		got, err := p.Pick(addrs)
		if err != nil {
			t.Fatal(err)
		}
		if got == "a" {
			t.Fatalf("a 已有 100 个在途请求，不应被选中（第 %d 次）", i)
		}
	}
	t.Log("P2C 避开高负载实例 OK")
}
