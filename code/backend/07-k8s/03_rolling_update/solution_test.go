package rolling_update

import (
	"reflect"
	"testing"
)

func TestRollingUpdateSurge(t *testing.T) {
	got := RollingUpdatePlan(3, 1, 1, 3)
	want := [][]Action{
		{{Op: "create", Version: "v2"}, {Op: "delete", Version: "v1"}, {Op: "delete", Version: "v1"}},
		{{Op: "create", Version: "v2"}, {Op: "create", Version: "v2"}, {Op: "delete", Version: "v1"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("maxSurge=1 滚动计划错误\n got: %v\nwant: %v", got, want)
	}
	t.Log("滚动更新（maxUnavailable=1/maxSurge=1）验证通过")
}

func TestRollingUpdateZeroDowntime(t *testing.T) {
	got := RollingUpdatePlan(3, 0, 1, 3)
	want := [][]Action{
		{{Op: "create", Version: "v2"}, {Op: "delete", Version: "v1"}},
		{{Op: "create", Version: "v2"}, {Op: "delete", Version: "v1"}},
		{{Op: "create", Version: "v2"}, {Op: "delete", Version: "v1"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("零宕机滚动计划错误\n got: %v\nwant: %v", got, want)
	}
	t.Log("零宕机滚动（maxUnavailable=0）验证通过")
}

func TestRollingUpdateNoSurge(t *testing.T) {
	got := RollingUpdatePlan(2, 1, 0, 2)
	want := [][]Action{
		{{Op: "delete", Version: "v1"}},
		{{Op: "create", Version: "v2"}, {Op: "delete", Version: "v1"}},
		{{Op: "create", Version: "v2"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("maxSurge=0 滚动计划错误\n got: %v\nwant: %v", got, want)
	}
	t.Log("无超配滚动（maxSurge=0）验证通过")
}

func TestScaleUpAndDown(t *testing.T) {
	// 纯扩容：v1=0，一次创建到目标
	up := RollingUpdatePlan(3, 1, 2, 0)
	wantUp := [][]Action{
		{{Op: "create", Version: "v2"}, {Op: "create", Version: "v2"}, {Op: "create", Version: "v2"}},
	}
	if !reflect.DeepEqual(up, wantUp) {
		t.Fatalf("纯扩容计划错误\n got: %v\nwant: %v", up, wantUp)
	}
	// 缩容到零：全删 v1
	down := RollingUpdatePlan(0, 1, 1, 2)
	wantDown := [][]Action{
		{{Op: "delete", Version: "v1"}, {Op: "delete", Version: "v1"}},
	}
	if !reflect.DeepEqual(down, wantDown) {
		t.Fatalf("缩容到零计划错误\n got: %v\nwant: %v", down, wantDown)
	}
	t.Log("纯扩容 / 缩容到零验证通过")
}
