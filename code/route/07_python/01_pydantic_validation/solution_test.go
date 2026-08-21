package pydantic_validation

import (
	"strings"
	"testing"
)

func TestParse_OK(t *testing.T) {
	task, err := ParseClipTask([]byte(`{"task_id": "t-1", "duration": 10, "tags": ["cut"]}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.TaskID != "t-1" || task.Duration != 10 {
		t.Fatalf("got %+v, want TaskID=t-1 Duration=10", task)
	}
	if len(task.Tags) != 1 || task.Tags[0] != "cut" {
		t.Fatalf("tags = %v, want [cut]", task.Tags)
	}
	if task.Title != nil { // 缺省应 nil
		t.Fatalf("Title = %v, want nil", *task.Title)
	}
}

func TestParse_MissingTaskID(t *testing.T) {
	// 缺 task_id → 零值 "" → 校验失败
	if _, err := ParseClipTask([]byte(`{"duration": 10}`)); err == nil {
		t.Fatal("缺 task_id 应报错")
	}
}

func TestParse_TaskIDWithSpace(t *testing.T) {
	if _, err := ParseClipTask([]byte(`{"task_id": "t 1", "duration": 10}`)); err == nil {
		t.Fatal("task_id 含空格应报错")
	}
}

func TestParse_TaskIDTooLong(t *testing.T) {
	long := strings.Repeat("a", 65)
	if _, err := ParseClipTask([]byte(`{"task_id": "` + long + `", "duration": 10}`)); err == nil {
		t.Fatal("task_id 超长应报错")
	}
}

func TestParse_MissingDuration(t *testing.T) {
	// 缺 duration → 零值 0 → 不满足 (0,600] → 报错（"必填"语义）
	if _, err := ParseClipTask([]byte(`{"task_id": "t-1"}`)); err == nil {
		t.Fatal("缺 duration 应报错")
	}
}

func TestParse_DurationTooLarge(t *testing.T) {
	if _, err := ParseClipTask([]byte(`{"task_id": "t-1", "duration": 601}`)); err == nil {
		t.Fatal("duration 越界应报错")
	}
}

func TestParse_TypeMismatch(t *testing.T) {
	// 对照考点 1：Go 不做类型转换，字符串进 int 字段直接报错（Pydantic 会转成 30）
	if _, err := ParseClipTask([]byte(`{"task_id": "t-1", "duration": "30"}`)); err == nil {
		t.Fatal("类型不匹配应报错")
	}
}

func TestParse_TitleEmptyString(t *testing.T) {
	// 指针区分：传了空串 → 非 nil，值等于 ""
	task, err := ParseClipTask([]byte(`{"task_id": "t-1", "duration": 10, "title": ""}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.Title == nil {
		t.Fatal("title 传了空串应非 nil")
	}
	if *task.Title != "" {
		t.Fatalf("Title = %q, want empty string", *task.Title)
	}
}
