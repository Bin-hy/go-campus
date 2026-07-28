package growth_predict

import (
	"reflect"
	"testing"
)

func TestPredictGrowth(t *testing.T) {
	tests := []struct {
		name     string
		oldCap   int
		needCap  int
		expected int
	}{
		{"需要容量超过2倍-直接用需要的", 4, 20, 20},
		{"小容量翻倍", 4, 5, 8},
		{"刚好翻倍", 4, 8, 8},
		{"零容量", 0, 1, 1},
		{"零容量需要多个", 0, 5, 5},
		{"256临界点-翻倍", 128, 200, 256},
		{"256临界点-增长公式", 256, 300, 448},  // 256 + (256 + 768)/4 = 256 + 256 = 512... 实际: 256 + (256+768)/4 = 256+256=512? No. newcap=256, newcap += (256+768)/4 = 256+256 = 512 >= 300. 所以是512
		{"大容量增长", 512, 600, 704},           // 512 + (512 + 768) / 4 = 512 + 320 = 832? No wait: newcap=512, newcap += (512+768)/4 = 512 + 320 = 832 >= 600, 所以832? 
	}
	
	// 修正预期值：重新计算
	// 256 -> needCap=300: newcap=256, newcap += (256+768)/4 = 256+256=512. 512 >= 300. result=512
	// 512 -> needCap=600: newcap=512, newcap += (512+768)/4 = 512+320=832. 832 >= 600. result=832
	tests[6].expected = 512
	tests[7].expected = 832

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PredictGrowth(tt.oldCap, tt.needCap)
			if result != tt.expected {
				t.Errorf("PredictGrowth(%d, %d) = %d，期望 %d",
					tt.oldCap, tt.needCap, result, tt.expected)
			}
		})
	}
}

func TestOptimalPrealloc(t *testing.T) {
	tests := []struct {
		name              string
		appendSizes       []int
		expectWithout     int
		expectWith        int
	}{
		{
			name:          "逐个append10次",
			appendSizes:   []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
			expectWithout: 4, // cap: 0->1->2->4->8->16, 扩容4次
			expectWith:    0, // 预分配10，不需要扩容
		},
		{
			name:          "一次性append",
			appendSizes:   []int{100},
			expectWithout: 1, // cap: 0->100, 扩容1次
			expectWith:    0,
		},
		{
			name:          "空操作",
			appendSizes:   []int{},
			expectWithout: 0,
			expectWith:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			without, with := OptimalPrealloc(tt.appendSizes)
			if without != tt.expectWithout {
				t.Errorf("不预分配扩容次数：期望%d，得到%d", tt.expectWithout, without)
			}
			if with != tt.expectWith {
				t.Errorf("预分配扩容次数：期望%d，得到%d", tt.expectWith, with)
			}
		})
	}
}

func TestBatchAppend(t *testing.T) {
	tests := []struct {
		name     string
		input    [][]int
		expected []int
	}{
		{
			name:     "多个切片合并",
			input:    [][]int{{1, 2}, {3, 4, 5}, {6}},
			expected: []int{1, 2, 3, 4, 5, 6},
		},
		{
			name:     "包含空切片",
			input:    [][]int{{1}, {}, {2, 3}, nil, {4}},
			expected: []int{1, 2, 3, 4},
		},
		{
			name:     "单个切片",
			input:    [][]int{{1, 2, 3}},
			expected: []int{1, 2, 3},
		},
		{
			name:     "全空",
			input:    [][]int{{}, {}, {}},
			expected: []int{},
		},
		{
			name:     "无输入",
			input:    nil,
			expected: []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BatchAppend(tt.input...)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("期望 %v，得到 %v", tt.expected, result)
			}
		})
	}
}

func TestBatchAppend_SingleAlloc(t *testing.T) {
	slices := [][]int{{1, 2, 3}, {4, 5}, {6, 7, 8, 9}}
	result := BatchAppend(slices...)

	// 总长度为 9，cap 也应该恰好为 9（只分配一次）
	totalLen := 0
	for _, s := range slices {
		totalLen += len(s)
	}

	if cap(result) != totalLen {
		t.Errorf("应只分配一次内存：期望 cap=%d，得到 cap=%d", totalLen, cap(result))
	}
}
