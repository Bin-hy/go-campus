package service_lb

type Pod struct {
	IP     string
	Labels map[string]string
	Ready  bool
	Conns  int
}

// BuildEndpoints 只有 label 完全匹配且 Ready 的 Pod 才进 Endpoints。
func BuildEndpoints(pods []Pod, selector map[string]string) []string {
	var out []string
	for _, p := range pods {
		if !p.Ready {
			continue
		}
		match := true
		for k, v := range selector {
			if p.Labels[k] != v {
				match = false
				break
			}
		}
		if match {
			out = append(out, p.IP)
		}
	}
	return out
}

// LoadBalancer 负载均衡器。
type LoadBalancer interface {
	Next() string
}

// --- 轮询（RoundRobin）---

type roundRobin struct {
	ips  []string
	next int
}

func (rr *roundRobin) Next() string {
	if len(rr.ips) == 0 {
		return ""
	}
	ip := rr.ips[rr.next%len(rr.ips)]
	rr.next++
	return ip
}

func NewRoundRobin(ips []string) LoadBalancer {
	return &roundRobin{ips: ips}
}

// --- 最少连接（LeastConnections）---

type leastConn struct {
	pods []Pod
}

func (lc *leastConn) Next() string {
	if len(lc.pods) == 0 {
		return ""
	}
	idx := 0
	for i := 1; i < len(lc.pods); i++ {
		if lc.pods[i].Conns < lc.pods[idx].Conns ||
			(lc.pods[i].Conns == lc.pods[idx].Conns && lc.pods[i].IP < lc.pods[idx].IP) {
			idx = i
		}
	}
	lc.pods[idx].Conns++ // 新连接挂上
	return lc.pods[idx].IP
}

func NewLeastConn(pods []Pod) LoadBalancer {
	// 复制一份，避免外部改动影响均衡器内部状态
	return &leastConn{pods: append([]Pod(nil), pods...)}
}
