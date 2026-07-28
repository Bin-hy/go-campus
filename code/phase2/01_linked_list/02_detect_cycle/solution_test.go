package detect_cycle

import "testing"

// --- 辅助函数 ---

// buildCycleList 构建带环的链表
// vals 为节点值数组，cyclePos 为环入口的索引（-1 表示无环）
func buildCycleList(vals []int, cyclePos int) (*ListNode, *ListNode) {
	if len(vals) == 0 {
		return nil, nil
	}
	nodes := make([]*ListNode, len(vals))
	for i, v := range vals {
		nodes[i] = &ListNode{Val: v}
	}
	for i := 0; i < len(nodes)-1; i++ {
		nodes[i].Next = nodes[i+1]
	}
	var cycleNode *ListNode
	if cyclePos >= 0 && cyclePos < len(nodes) {
		nodes[len(nodes)-1].Next = nodes[cyclePos]
		cycleNode = nodes[cyclePos]
	}
	return nodes[0], cycleNode
}

// --- 测试用例 ---

func TestDetectCycle(t *testing.T) {
	tests := []struct {
		name     string
		vals     []int
		cyclePos int
	}{
		{
			name:     "环在第二个节点",
			vals:     []int{3, 2, 0, -4},
			cyclePos: 1,
		},
		{
			name:     "环在头节点",
			vals:     []int{1, 2, 3},
			cyclePos: 0,
		},
		{
			name:     "环在尾节点（自环）",
			vals:     []int{1, 2, 3},
			cyclePos: 2,
		},
		{
			name:     "无环",
			vals:     []int{1, 2, 3, 4, 5},
			cyclePos: -1,
		},
		{
			name:     "单节点无环",
			vals:     []int{1},
			cyclePos: -1,
		},
		{
			name:     "单节点自环",
			vals:     []int{1},
			cyclePos: 0,
		},
		{
			name:     "两节点有环",
			vals:     []int{1, 2},
			cyclePos: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			head, expected := buildCycleList(tt.vals, tt.cyclePos)
			result := detectCycle(head)

			if result != expected {
				if expected == nil {
					t.Errorf("期望无环(nil)，得到节点 val=%d", result.Val)
				} else if result == nil {
					t.Errorf("期望环入口 val=%d，得到 nil", expected.Val)
				} else {
					t.Errorf("期望环入口 val=%d，得到 val=%d", expected.Val, result.Val)
				}
			}
		})
	}
}

func TestDetectCycle_Nil(t *testing.T) {
	result := detectCycle(nil)
	if result != nil {
		t.Errorf("detectCycle(nil) 应返回 nil")
	}
}
