// Package answer 参考答案（自包含：不依赖父包，可独立编译对照阅读）。
package answer

import (
	"os"
	"strings"
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

// BuildClient 从 kubeconfig 构建 clientset。
func BuildClient() (*kubernetes.Clientset, error) {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = clientcmd.RecommendedHomeFile
	}
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

type EventType string

const (
	EventAdded    EventType = "Added"
	EventModified EventType = "Modified"
	EventDeleted  EventType = "Deleted"
)

type PodEvent struct {
	Type   EventType
	Pod    string
	Reason string
}

func DescribeEvent(e EventType) string {
	switch e {
	case EventAdded:
		return "新 Pod 加入"
	case EventModified:
		return "Pod 配置/状态变化"
	case EventDeleted:
		return "Pod 消失"
	default:
		return "未知事件"
	}
}

func ClassifyEvent(raw string) EventType {
	switch strings.ToUpper(raw) {
	case "ADDED":
		return EventAdded
	case "MODIFIED":
		return EventModified
	case "DELETED":
		return EventDeleted
	default:
		return EventType(raw)
	}
}

func Summarize(events []string) []PodEvent {
	out := make([]PodEvent, 0, len(events))
	for _, raw := range events {
		parts := strings.SplitN(raw, " ", 2)
		typ := ClassifyEvent(parts[0])
		pod := ""
		if len(parts) > 1 {
			pod = parts[1]
		}
		out = append(out, PodEvent{Type: typ, Pod: pod, Reason: DescribeEvent(typ)})
	}
	return out
}

func BuildInformer(client kubernetes.Interface, namespace string) (cache.SharedIndexInformer, error) {
	factory := informers.NewSharedInformerFactoryWithOptions(client, 30*time.Second,
		informers.WithNamespace(namespace))
	return factory.Core().V1().Pods().Informer(), nil
}
