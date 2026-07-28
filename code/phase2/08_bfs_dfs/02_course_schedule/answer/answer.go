//go:build ignore

package answer

// BFS 拓扑排序（Kahn 算法）
func canFinish(numCourses int, prerequisites [][]int) bool {
	graph := make([][]int, numCourses)
	inDegree := make([]int, numCourses)

	for _, pre := range prerequisites {
		a, b := pre[0], pre[1]
		graph[b] = append(graph[b], a)
		inDegree[a]++
	}

	queue := make([]int, 0)
	for i := 0; i < numCourses; i++ {
		if inDegree[i] == 0 {
			queue = append(queue, i)
		}
	}

	count := 0
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		count++
		for _, next := range graph[cur] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}
	return count == numCourses
}
