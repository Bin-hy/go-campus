package main

import (
	"context"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestListPodNames(t *testing.T) {
	client := fake.NewSimpleClientset(
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-a", Namespace: "default"}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-b", Namespace: "default"}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-c", Namespace: "kube-system"}},
	)

	got, err := ListPodNames(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("ListPodNames err = %v", err)
	}
	want := []string{"pod-a", "pod-b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListPodNames(default) = %v, want %v", got, want)
	}
	t.Log("命名空间过滤验证通过")
}
