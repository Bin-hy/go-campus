package merge_k_sorted

import (
	"fmt"
	"testing"
)

// --- 辅助函数 ---

func buildList(vals []int) *ListNode {
	dummy := &ListNode{}
	cur := dummy
	for _, v := range vals {
		cur.Next = &ListNode{Val: v}
		cur = cur.Next
	}
	return dummy.Next
}

func toSlice(head *ListNode) []int {
	var result []int
	for cur := head; cur != nil; cur = cur.Next {
		result = append(result, cur.Val)
	}
	return result
}

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

func TestMergeKLists(t *testing.T) {
	tests := []struct {
		name     string
		lists    [][]int
		expected []int
	}{
		{
			name:     "三个链表",
			lists:    [][]int{{1, 4, 5}, {1, 3, 4}, {2, 6}},
			expected: []int{1, 1, 2, 3, 4, 4, 5, 6},
		},
		{
			name:     "空数组",
			lists:    [][]int{},
			expected: []int{},
		},
		{
			name:     "含一个空链表",
			lists:    [][]int{{}},
			expected: []int{},
		},
		{
			name:     "只有一个链表",
			lists:    [][]int{{1, 2, 3}},
			expected: []int{1, 2, 3},
		},
		{
			name:     "两个链表",
			lists:    [][]int{{1, 3, 5}, {2, 4, 6}},
			expected: []int{1, 2, 3, 4, 5, 6},
		},
		{
			name:     "包含负数",
			lists:    [][]int{{-10, -5, 0}, {-3, 1, 4}, {-8, 2}},
			expected: []int{-10, -8, -5, -3, 0, 1, 2, 4},
		},
		{
			name:     "多个空链表混合",
			lists:    [][]int{{}, {1}, {}, {2, 3}},
			expected: []int{1, 2, 3},
		},
		{
			name:     "所有元素相同",
			lists:    [][]int{{1, 1}, {1, 1}, {1}},
			expected: []int{1, 1, 1, 1, 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lists := make([]*ListNode, len(tt.lists))
			for i, vals := range tt.lists {
				lists[i] = buildList(vals)
			}
			result := mergeKLists(lists)
			got := toSlice(result)

			if len(tt.expected) == 0 {
				if len(got) != 0 {
					t.Errorf("期望空链表，得到 %v", got)
				}
				return
			}

			if !equal(got, tt.expected) {
				t.Errorf("mergeKLists(%v) = %v, 期望 %v", tt.lists, got, tt.expected)
			}
		})
	}
}

func TestMergeKLists_Nil(t *testing.T) {
	result := mergeKLists(nil)
	if result != nil {
		t.Errorf("mergeKLists(nil) 应返回 nil，得到 %v", toSlice(result))
	}
}

func Example_mergeKLists() {
	lists := []*ListNode{
		buildList([]int{1, 4, 5}),
		buildList([]int{1, 3, 4}),
		buildList([]int{2, 6}),
	}
	result := mergeKLists(lists)
	fmt.Println(toSlice(result))
	// Output: [1 1 2 3 4 4 5 6]
}
