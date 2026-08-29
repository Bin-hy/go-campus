package main

import (
	"reflect"
	"testing"
)

func TestClassifyEvent(t *testing.T) {
	cases := []struct{ raw, want string }{
		{"ADDED", "Added"},
		{"modified", "Modified"},
		{"DELETED", "Deleted"},
	}
	for _, c := range cases {
		if got := ClassifyEvent(c.raw); string(got) != c.want {
			t.Errorf("ClassifyEvent(%q) = %s, want %s", c.raw, got, c.want)
		}
	}
	t.Log("事件分类验证通过")
}

func TestDescribeEvent(t *testing.T) {
	if DescribeEvent(EventAdded) == "" || DescribeEvent(EventModified) == "" || DescribeEvent(EventDeleted) == "" {
		t.Fatal("三种事件都应有说明文字")
	}
	t.Log("事件说明验证通过")
}

func TestSummarize(t *testing.T) {
	got := Summarize([]string{"ADDED pod-1", "MODIFIED pod-1", "DELETED pod-1"})
	want := []PodEvent{
		{Type: EventAdded, Pod: "pod-1", Reason: "新 Pod 加入"},
		{Type: EventModified, Pod: "pod-1", Reason: "Pod 配置/状态变化"},
		{Type: EventDeleted, Pod: "pod-1", Reason: "Pod 消失"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Summarize = %+v, want %+v", got, want)
	}
	t.Log("事件汇总验证通过")
}
