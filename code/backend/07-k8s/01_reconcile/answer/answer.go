package reconcile

import (
	"sort"
	"strconv"
	"strings"
)

type Pod struct {
	Name string
	Age  int
}

type Action struct {
	Op   string
	Name string
}

func DesiredReplicas(currentReplicas, avgCPU, targetUtil, minReplicas, maxReplicas int) int {
	if avgCPU == 0 {
		return minReplicas
	}
	desired := (currentReplicas*avgCPU + targetUtil - 1) / targetUtil // 向上取整
	if desired < minReplicas {
		return minReplicas
	}
	if desired > maxReplicas {
		return maxReplicas
	}
	return desired
}

func Reconcile(desired int, pods []Pod) []Action {
	var actions []Action

	if len(pods) < desired {
		// 扩容：名字从现有最大编号 +1 开始
		maxSuffix := 0
		for _, p := range pods {
			if n, err := strconv.Atoi(strings.TrimPrefix(p.Name, "pod-")); err == nil && n > maxSuffix {
				maxSuffix = n
			}
		}
		for i := 0; i < desired-len(pods); i++ {
			maxSuffix++
			actions = append(actions, Action{Op: "create", Name: "pod-" + strconv.Itoa(maxSuffix)})
		}
		return actions
	}

	if len(pods) > desired {
		// 缩容：先删最老的（Age 最小），Age 相同按名字字典序
		sorted := append([]Pod(nil), pods...)
		sort.SliceStable(sorted, func(i, j int) bool {
			if sorted[i].Age != sorted[j].Age {
				return sorted[i].Age < sorted[j].Age
			}
			return sorted[i].Name < sorted[j].Name
		})
		for i := 0; i < len(pods)-desired; i++ {
			actions = append(actions, Action{Op: "delete", Name: sorted[i].Name})
		}
		return actions
	}

	return actions
}
