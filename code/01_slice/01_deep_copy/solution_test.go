package deep_copy

import (
	"reflect"
	"testing"
)

func TestDeepCopy(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		expected []int
		isNil    bool
	}{
		{
			name:     "正常切片",
			input:    []int{1, 2, 3, 4, 5},
			expected: []int{1, 2, 3, 4, 5},
		},
		{
			name:  "nil切片返回nil",
			input: nil,
			isNil: true,
		},
		{
			name:     "空切片返回空切片",
			input:    []int{},
			expected: []int{},
		},
		{
			name:     "单元素",
			input:    []int{42},
			expected: []int{42},
		},
		{
			name:     "包含负数和零",
			input:    []int{-1, 0, 1, -100, 100},
			expected: []int{-1, 0, 1, -100, 100},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DeepCopy(tt.input)

			if tt.isNil {
				if result != nil {
					t.Errorf("期望返回 nil，得到 %v", result)
				}
				return
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("期望 %v，得到 %v", tt.expected, result)
			}
		})
	}
}

func TestDeepCopy_Independence(t *testing.T) {
	src := []int{1, 2, 3, 4, 5}
	dst := DeepCopy(src)

	// 修改副本不应影响原切片
	dst[0] = 100
	if src[0] != 1 {
		t.Errorf("深拷贝失败：修改副本影响了原切片，src[0]=%d，期望1", src[0])
	}

	// 修改原切片不应影响副本
	src[1] = 200
	if dst[1] != 2 {
		t.Errorf("深拷贝失败：修改原切片影响了副本，dst[1]=%d，期望2", dst[1])
	}
}

func TestDeepCopy_EmptyNotNil(t *testing.T) {
	src := []int{}
	result := DeepCopy(src)

	if result == nil {
		t.Error("空切片的深拷贝不应返回 nil")
	}
	if len(result) != 0 {
		t.Errorf("空切片的深拷贝长度应为0，得到 %d", len(result))
	}
}
