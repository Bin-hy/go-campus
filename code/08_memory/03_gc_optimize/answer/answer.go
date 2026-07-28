//go:build ignore

package answer

import (
	"strings"
	"sync"
)

func ProcessData_Good(inputs []int) []int {
	results := make([]int, len(inputs)) // 一次性分配
	for i, v := range inputs {
		results[i] = v * 2
	}
	return results
}

func ConcatStrings_Good(strs []string) string {
	var builder strings.Builder
	for _, s := range strs {
		builder.WriteString(s)
	}
	return builder.String()
}

func ObjectPool_Demo(n int) int {
	pool := sync.Pool{
		New: func() interface{} {
			buf := make([]byte, 1024)
			return &buf
		},
	}

	count := 0
	for i := 0; i < n; i++ {
		buf := pool.Get().(*[]byte)
		// 模拟处理
		(*buf)[0] = byte(i)
		count++
		pool.Put(buf)
	}
	return count
}
