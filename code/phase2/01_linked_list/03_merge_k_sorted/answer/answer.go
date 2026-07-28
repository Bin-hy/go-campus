//go:build ignore

package answer

type ListNode struct {
	Val  int
	Next *ListNode
}

// 分治法：两两归并
func mergeKLists(lists []*ListNode) *ListNode {
	if len(lists) == 0 {
		return nil
	}
	return divideAndConquer(lists, 0, len(lists)-1)
}

func divideAndConquer(lists []*ListNode, left, right int) *ListNode {
	if left == right {
		return lists[left]
	}
	mid := left + (right-left)/2
	l := divideAndConquer(lists, left, mid)
	r := divideAndConquer(lists, mid+1, right)
	return mergeTwoLists(l, r)
}

func mergeTwoLists(l1, l2 *ListNode) *ListNode {
	dummy := &ListNode{}
	cur := dummy
	for l1 != nil && l2 != nil {
		if l1.Val <= l2.Val {
			cur.Next = l1
			l1 = l1.Next
		} else {
			cur.Next = l2
			l2 = l2.Next
		}
		cur = cur.Next
	}
	if l1 != nil {
		cur.Next = l1
	} else {
		cur.Next = l2
	}
	return dummy.Next
}
