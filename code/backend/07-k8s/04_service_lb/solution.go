package service_lb

// Pod 集群里的一个 Pod。
type Pod struct {
	IP     string
	Labels map[string]string
	Ready  bool
	Conns  int // 当前连接数
}

// BuildEndpoints 选出匹配 selector 且 Ready 的 Pod，返回 IP 列表。
func BuildEndpoints(pods []Pod, selector map[string]string) []string {
	// TODO: 实现你的代码
	panic("not implemented")
}

// LoadBalancer 负载均衡器。
type LoadBalancer interface {
	Next() string
}

// NewRoundRobin 创建一个轮询负载均衡器。
func NewRoundRobin(ips []string) LoadBalancer {
	// TODO: 实现你的代码
	panic("not implemented")
}

// NewLeastConn 创建一个最少连接负载均衡器。
func NewLeastConn(pods []Pod) LoadBalancer {
	// TODO: 实现你的代码
	panic("not implemented")
}
