// Package answer 参考答案（自包含：不依赖父包，可独立编译对照阅读）。
package answer

import (
	"context"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BuildClient 从 kubeconfig 构建 clientset（KUBECONFIG 优先，默认 ~/.kube/config）。
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

// ListPodNames 列出指定 namespace 的 Pod 名。
func ListPodNames(ctx context.Context, client kubernetes.Interface, namespace string) ([]string, error) {
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(pods.Items))
	for _, p := range pods.Items {
		names = append(names, p.Name)
	}
	return names, nil
}
