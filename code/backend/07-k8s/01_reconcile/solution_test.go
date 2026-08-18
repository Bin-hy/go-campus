package reconcile

import (
	"reflect"
	"testing"
)

func TestDesiredReplicas(t *testing.T) {
	cases := []struct {
		name                              string
		current, avgCPU, target, min, max int
		want                              int
	}{
		{"压力上升扩容", 10, 80, 60, 2, 20, 14},
		{"压力下降缩容", 10, 20, 60, 2, 20, 4},
		{"无压力回到下限", 5, 0, 60, 2, 20, 2},
		{"超过上限钳制", 10, 500, 60, 2, 20, 20},
		{"低于下限钳制", 1, 1, 60, 2, 20, 2},
		{"刚好等于目标", 3, 60, 60, 1, 10, 3},
	}
	for _, c := range cases {
		got := DesiredReplicas(c.current, c.avgCPU, c.target, c.min, c.max)
		if got != c.want {
			t.Errorf("%s: DesiredReplicas(%d,%d,%d,%d,%d)=%d, want %d",
				c.name, c.current, c.avgCPU, c.target, c.min, c.max, got, c.want)
		}
	}
	t.Log("HPA 期望副本数计算验证通过")
}

func TestReconcileScaleUp(t *testing.T) {
	pods := []Pod{{Name: "pod-1", Age: 100}, {Name: "pod-2", Age: 50}}
	got := Reconcile(3, pods)
	want := []Action{{Op: "create", Name: "pod-3"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Reconcile(3, [pod-1 pod-2]) = %v, want %v", got, want)
	}
	t.Log("扩容：补建 1 个 pod-3 验证通过")
}

func TestReconcileScaleDown(t *testing.T) {
	pods := []Pod{
		{Name: "pod-1", Age: 100},
		{Name: "pod-2", Age: 50},
		{Name: "pod-3", Age: 80},
	}
	got := Reconcile(1, pods)
	want := []Action{{Op: "delete", Name: "pod-2"}, {Op: "delete", Name: "pod-3"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Reconcile(1, ...) = %v, want %v", got, want)
	}
	t.Log("缩容：先删最老的两个验证通过")
}

func TestReconcileTieBreak(t *testing.T) {
	pods := []Pod{{Name: "pod-b", Age: 50}, {Name: "pod-a", Age: 50}, {Name: "pod-c", Age: 100}}
	got := Reconcile(1, pods)
	want := []Action{{Op: "delete", Name: "pod-a"}, {Op: "delete", Name: "pod-b"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Age 相同应按名字字典序删，实际 %v, want %v", got, want)
	}
	t.Log("Age 相同时按名字字典序验证通过")
}

func TestReconcileNoop(t *testing.T) {
	pods := []Pod{{Name: "pod-1", Age: 100}, {Name: "pod-2", Age: 50}}
	if got := Reconcile(2, pods); len(got) != 0 {
		t.Fatalf("状态一致时应无动作，实际 %v", got)
	}
	t.Log("状态一致：无动作验证通过")
}
