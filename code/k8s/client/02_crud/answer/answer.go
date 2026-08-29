// Package answer 参考答案（自包含：不依赖父包，可独立编译对照阅读）。
package answer

import (
	"context"
	"os"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"
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

// CreateDeployment 创建 2 副本 nginx Deployment。
func CreateDeployment(ctx context.Context, client kubernetes.Interface, ns, name string) (*appsv1.Deployment, error) {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    map[string]string{"app": name},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(int32(2)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:1.27",
							Ports: []corev1.ContainerPort{{ContainerPort: 80}},
						},
					},
				},
			},
		},
	}
	return client.AppsV1().Deployments(ns).Create(ctx, deploy, metav1.CreateOptions{})
}

// ScaleDeployment 改副本数。
func ScaleDeployment(ctx context.Context, client kubernetes.Interface, ns, name string, replicas int32) (*appsv1.Deployment, error) {
	deploy, err := client.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	deploy.Spec.Replicas = &replicas
	return client.AppsV1().Deployments(ns).Update(ctx, deploy, metav1.UpdateOptions{})
}

// DeleteDeployment 立即删除。
func DeleteDeployment(ctx context.Context, client kubernetes.Interface, ns, name string) error {
	return client.AppsV1().Deployments(ns).Delete(ctx, name, metav1.DeleteOptions{GracePeriodSeconds: ptr.To(int64(0))})
}

// ListDeployments 列出名字。
func ListDeployments(ctx context.Context, client kubernetes.Interface, ns string) ([]string, error) {
	list, err := client.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(list.Items))
	for _, d := range list.Items {
		names = append(names, d.Name)
	}
	return names, nil
}
