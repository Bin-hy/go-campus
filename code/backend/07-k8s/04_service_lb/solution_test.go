package service_lb

import (
	"reflect"
	"testing"
)

func TestBuildEndpoints(t *testing.T) {
	pods := []Pod{
		{IP: "10.0.0.1", Labels: map[string]string{"app": "go-app", "env": "prod"}, Ready: true},
		{IP: "10.0.0.2", Labels: map[string]string{"app": "go-app", "env": "prod"}, Ready: false}, // 未就绪，剔除
		{IP: "10.0.0.3", Labels: map[string]string{"app": "other"}, Ready: true},                  // 不匹配
		{IP: "10.0.0.4", Labels: map[string]string{"app": "go-app"}, Ready: true},                 // 缺 env 标签，不匹配
	}
	got := BuildEndpoints(pods, map[string]string{"app": "go-app", "env": "prod"})
	want := []string{"10.0.0.1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Endpoints 应为 [10.0.0.1]，实际 %v", got)
	}
	t.Log("label selector + readiness 摘除验证通过")
}

func TestRoundRobin(t *testing.T) {
	lb := NewRoundRobin([]string{"10.0.0.1", "10.0.0.2", "10.0.0.3"})
	seq := []string{}
	for i := 0; i < 5; i++ {
		seq = append(seq, lb.Next())
	}
	want := []string{"10.0.0.1", "10.0.0.2", "10.0.0.3", "10.0.0.1", "10.0.0.2"}
	if !reflect.DeepEqual(seq, want) {
		t.Fatalf("轮询序列应为 %v，实际 %v", want, seq)
	}
	if NewRoundRobin(nil).Next() != "" {
		t.Fatal("空 Endpoints 应返回空串")
	}
	t.Log("轮询转发验证通过")
}

func TestLeastConn(t *testing.T) {
	pods := []Pod{
		{IP: "10.0.0.1", Conns: 3},
		{IP: "10.0.0.2", Conns: 1},
		{IP: "10.0.0.3", Conns: 2},
	}
	lb := NewLeastConn(pods)
	// 第一次：连接数最少的是 10.0.0.2（1），选中后变 2
	if got := lb.Next(); got != "10.0.0.2" {
		t.Fatalf("第一次应选 10.0.0.2，实际 %s", got)
	}
	// 第二次：10.0.0.2(2) 与 10.0.0.3(2) 平局，按 IP 字典序取 10.0.0.2
	if got := lb.Next(); got != "10.0.0.2" {
		t.Fatalf("平局应取字典序小的 10.0.0.2，实际 %s", got)
	}
	t.Log("最少连接转发验证通过")
}
