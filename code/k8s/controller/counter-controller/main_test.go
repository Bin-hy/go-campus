package main

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// newTestController 用 fake clientset 构建控制器（不启动 informer，直接测 reconcile）。
func newTestController(cm *corev1.ConfigMap) (*Controller, *fake.Clientset) {
	client := fake.NewSimpleClientset()
	if cm != nil {
		client = fake.NewSimpleClientset(cm)
	}
	c := &Controller{client: client}
	return c, client
}

func TestReconcileCreateDeployment(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "web",
			Namespace: "default",
			Annotations: map[string]string{
				annotationKey: "3",
			},
		},
	}
	c, client := newTestController(cm)

	if err := c.reconcile(context.Background(), "default/web"); err != nil {
		t.Fatalf("reconcile err = %v", err)
	}

	deploy, err := client.AppsV1().Deployments("default").Get(context.Background(), "web", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Deployment 应被创建: %v", err)
	}
	if deploy.Spec.Replicas == nil || *deploy.Spec.Replicas != 3 {
		t.Fatalf("副本数应为 3，实际 %v", deploy.Spec.Replicas)
	}
	t.Log("创建 Deployment 并设置副本数验证通过")
}

func TestReconcileScaleToMatch(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "web",
			Namespace: "default",
			Annotations: map[string]string{
				annotationKey: "5",
			},
		},
	}
	c, client := newTestController(cm)

	// 先手动建一个 2 副本的 Deployment（模拟"实际状态与期望不一致"）
	_, err := client.AppsV1().Deployments("default").Create(context.Background(),
		appsv1Deployment("default", "web", 2), metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("预置 Deployment 失败: %v", err)
	}

	if err := c.reconcile(context.Background(), "default/web"); err != nil {
		t.Fatalf("reconcile err = %v", err)
	}

	deploy, _ := client.AppsV1().Deployments("default").Get(context.Background(), "web", metav1.GetOptions{})
	if deploy.Spec.Replicas == nil || *deploy.Spec.Replicas != 5 {
		t.Fatalf("reconcile 后副本数应为 5，实际 %v", deploy.Spec.Replicas)
	}
	t.Log("收敛副本数验证通过")
}

func TestReconcileNoAnnotationIgnored(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "plain", Namespace: "default"},
	}
	c, client := newTestController(cm)

	if err := c.reconcile(context.Background(), "default/plain"); err != nil {
		t.Fatalf("无注解应直接忽略: %v", err)
	}
	_, err := client.AppsV1().Deployments("default").Get(context.Background(), "plain", metav1.GetOptions{})
	if err == nil {
		t.Fatal("无注解 ConfigMap 不应创建 Deployment")
	}
	t.Log("无注解忽略验证通过")
}

func TestReconcileDeleteFollows(t *testing.T) {
	// ConfigMap 不存在 → 应删除同名 Deployment
	c, client := newTestController(nil)
	_, _ = client.AppsV1().Deployments("default").Create(context.Background(),
		appsv1Deployment("default", "gone", 2), metav1.CreateOptions{})

	if err := c.reconcile(context.Background(), "default/gone"); err != nil {
		t.Fatalf("reconcile err = %v", err)
	}
	_, err := client.AppsV1().Deployments("default").Get(context.Background(), "gone", metav1.GetOptions{})
	if err == nil {
		t.Fatal("ConfigMap 删除后 Deployment 应被跟随删除")
	}
	t.Log("跟随删除验证通过")
}

func TestReconcileInvalidAnnotation(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bad",
			Namespace: "default",
			Annotations: map[string]string{
				annotationKey: "not-a-number",
			},
		},
	}
	c, _ := newTestController(cm)

	if err := c.reconcile(context.Background(), "default/bad"); err == nil {
		t.Fatal("非法注解应返回错误触发重试")
	}
	t.Log("非法注解报错验证通过")
}
