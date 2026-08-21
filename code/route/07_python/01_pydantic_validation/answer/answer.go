//go:build ignore

package answer

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ClipTask 与 solution.go 保持一致
type ClipTask struct {
	TaskID   string   `json:"task_id"`
	Duration int      `json:"duration"`
	Tags     []string `json:"tags,omitempty"`
	Title    *string  `json:"title,omitempty"`
}

// ErrInvalid 校验失败统一错误
var ErrInvalid = errors.New("invalid clip task")

// ParseClipTask 参考答案：json.Unmarshal 做映射（类型不匹配会返回 error），
// 再手动校验约束——"必填"通过"零值不满足约束"体现。
func ParseClipTask(raw []byte) (ClipTask, error) {
	var t ClipTask
	if err := json.Unmarshal(raw, &t); err != nil {
		return ClipTask{}, err // JSON 语法错误 / 类型不匹配
	}
	if len(t.TaskID) < 1 || len(t.TaskID) > 64 {
		return ClipTask{}, fmt.Errorf("%w: task_id 长度必须 1~64", ErrInvalid)
	}
	if strings.Contains(t.TaskID, " ") {
		return ClipTask{}, fmt.Errorf("%w: task_id 不能包含空格", ErrInvalid)
	}
	if t.Duration <= 0 || t.Duration > 600 {
		return ClipTask{}, fmt.Errorf("%w: duration 必须 (0,600]", ErrInvalid)
	}
	return t, nil
}
