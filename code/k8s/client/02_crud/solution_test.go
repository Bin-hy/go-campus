package main

import (
	"context"
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCreateAndListDeployment(t *testing.T) {
	client := fake.NewSimpleClientset()
	ctx := context.Background()

	d, err := CreateDeployment(ctx, client, "default", "web")
	if err != nil {
		t.Fatalf("CreateDeployment err = %v", err)
	}
	if d == nil || *d.Spec.Replicas != 2 {
		t.Fatalf("创建的 Deployment 副本数应为 2，实际 %v", d)
	}

	names, err := ListDeployments(ctx, client, "default")
	if err != nil {
		t.Fatalf("ListDeployments err = %v", err)
	}
	if !reflect.DeepEqual(names, []string{"web"}) {
		t.Fatalf("ListDeployments = %v, want [web]", names)
	}
	t.Log("创建 + 列表验证通过")
}

func TestScaleDeployment(t *testing.T) {
	client := fake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec:       appsv1.DeploymentSpec{Replicas: ptr2(2)},
	})
	ctx := context.Background()

	updated, err := ScaleDeployment(ctx, client, "default", "web", 5)
	if err != nil {
		t.Fatalf("ScaleDeployment err = %v", err)
	}
	if updated.Spec.Replicas == nil || *updated.Spec.Replicas != 5 {
		t.Fatalf("缩容后副本数应为 5，实际 %v", updated.Spec.Replicas)
	}
	t.Log("扩缩容验证通过")
}

func TestDeleteDeployment(t *testing.T) {
	client := fake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
	})
	ctx := context.Background()

	if err := DeleteDeployment(ctx, client, "default", "web"); err != nil {
		t.Fatalf("DeleteDeployment err = %v", err)
	}
	names, err := ListDeployments(ctx, client, "default")
	if err != nil {
		t.Fatalf("ListDeployments err = %v", err)
	}
	if len(names) != 0 {
		t.Fatalf("删除后应无 Deployment，实际 %v", names)
	}
	t.Log("删除验证通过")
}

func ptr2(v int32) *int32 { return &v }
