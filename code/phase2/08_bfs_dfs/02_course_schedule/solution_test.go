package course_schedule

import "testing"

func TestCanFinish(t *testing.T) {
	tests := []struct {
		name          string
		numCourses    int
		prerequisites [][]int
		expected      bool
	}{
		{
			name:          "简单依赖-可完成",
			numCourses:    2,
			prerequisites: [][]int{{1, 0}},
			expected:      true,
		},
		{
			name:          "环-不可完成",
			numCourses:    2,
			prerequisites: [][]int{{1, 0}, {0, 1}},
			expected:      false,
		},
		{
			name:          "无依赖",
			numCourses:    3,
			prerequisites: [][]int{},
			expected:      true,
		},
		{
			name:          "链式依赖",
			numCourses:    4,
			prerequisites: [][]int{{1, 0}, {2, 1}, {3, 2}},
			expected:      true,
		},
		{
			name:          "复杂有环",
			numCourses:    4,
			prerequisites: [][]int{{1, 0}, {2, 1}, {3, 2}, {1, 3}},
			expected:      false,
		},
		{
			name:          "多条路径无环",
			numCourses:    5,
			prerequisites: [][]int{{1, 0}, {2, 0}, {3, 1}, {3, 2}, {4, 3}},
			expected:      true,
		},
		{
			name:          "单门课",
			numCourses:    1,
			prerequisites: [][]int{},
			expected:      true,
		},
		{
			name:          "自环",
			numCourses:    3,
			prerequisites: [][]int{{0, 0}},
			expected:      false,
		},
		{
			name:          "大三角环",
			numCourses:    3,
			prerequisites: [][]int{{0, 1}, {1, 2}, {2, 0}},
			expected:      false,
		},
		{
			name:          "独立子图无环",
			numCourses:    6,
			prerequisites: [][]int{{1, 0}, {3, 2}, {5, 4}},
			expected:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := canFinish(tt.numCourses, tt.prerequisites)
			if got != tt.expected {
				t.Errorf("canFinish(%d, %v) = %v, 期望 %v",
					tt.numCourses, tt.prerequisites, got, tt.expected)
			}
		})
	}
}
