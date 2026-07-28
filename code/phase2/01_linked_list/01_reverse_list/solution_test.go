package reverse_list

import (
	"fmt"
	"testing"
)

// --- 辅助函数 ---

// buildList 从数组构建链表
func buildList(vals []int) *ListNode {
	dummy := &ListNode{}
	cur := dummy
	for _, v := range vals {
		cur.Next = &ListNode{Val: v}
		cur = cur.Next
	}
	return dummy.Next
}

// toSlice 链表转数组
func toSlice(head *ListNode) []int {
	var result []int
	for cur := head; cur != nil; cur = cur.Next {
		result = append(result, cur.Val)
	}
	return result
}

// equal 比较两个 int 切片是否相等
func equal(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// --- 测试用例 ---

func TestReverseList(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		expected []int
	}{
		{
			name:     "正常链表",
			input:    []int{1, 2, 3, 4, 5},
			expected: []int{5, 4, 3, 2, 1},
		},
		{
			name:     "两个节点",
			input:    []int{1, 2},
			expected: []int{2, 1},
		},
		{
			name:     "单个节点",
			input:    []int{1},
			expected: []int{1},
		},
		{
			name:     "空链表",
			input:    []int{},
			expected: []int{},
		},
		{
			name:     "包含负数",
			input:    []int{-1, 0, 1, 2, -3},
			expected: []int{-3, 2, 1, 0, -1},
		},
		{
			name:     "包含重复值",
			input:    []int{1, 1, 2, 2, 3},
			expected: []int{3, 2, 2, 1, 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			head := buildList(tt.input)
			result := reverseList(head)
			got := toSlice(result)

			if !equal(got, tt.expected) {
				t.Errorf("reverseList(%v) = %v, 期望 %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestReverseList_Nil(t *testing.T) {
	result := reverseList(nil)
	if result != nil {
		t.Errorf("reverseList(nil) 应返回 nil，得到 %v", toSlice(result))
	}
}

func Example_reverseList() {
	head := buildList([]int{1, 2, 3, 4, 5})
	result := reverseList(head)
	fmt.Println(toSlice(result))
	// Output: [5 4 3 2 1]
}
