// Package main 一个手写的 controller-runtime Operator（无需 kubebuilder 脚手架）。
//
// 管理 WebApp 自定义资源（CRD 见 manifests/10_crd/crd-webapp.yaml）：
//   期望状态 = WebApp.Spec（replicas + image）
//   实际状态 = 同名 Deployment
//   Reconcile 确保 Deployment 收敛到 spec，并把 readyReplicas 写回 WebApp.Status
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	webappv1 "gocampus/k8s/operator/api/v1"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(webappv1.AddToScheme(scheme))
}

// WebAppReconciler 实现 reconcile.Reconciler。
type WebAppReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile 核心逻辑：WebApp → 期望 Deployment；Deployment 变化 → 回写 status。
func (r *WebAppReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("reconcile", "webapp", req.NamespacedName)

	// 1. 读取 WebApp；不存在说明被删了，无需动作
	var wa webappv1.WebApp
	if err := r.Get(ctx, req.NamespacedName, &wa); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	// 2. 构造期望 Deployment（复用 reconcile 模式：读现状 → 算差距 → 补差距）
	deploy := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: wa.Name, Namespace: wa.Namespace}}
	result, err := controllerutil.CreateOrUpdate(ctx, r.Client, deploy, func() error {
		deploy.Labels = map[string]string{"app": wa.Name, "managed-by": "webapp-operator"}
		deploy.Spec = appsv1.DeploymentSpec{
			Replicas: &wa.Spec.Replicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": wa.Name}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": wa.Name}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "app", Image: wa.Spec.Image},
					},
				},
			},
		}
		return nil
	})
	if err != nil {
		return reconcile.Result{}, err
	}
	logger.Info("deployment reconciled", "op", result)

	// 3. 回写 status（读取实际 Deployment 的 readyReplicas）
	ready := deploy.Status.ReadyReplicas
	if wa.Status.ReadyReplicas != ready {
		wa.Status.ReadyReplicas = ready
		if err := r.Status().Update(ctx, &wa); err != nil {
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}

// SetupWithManager 注册：监听 WebApp 变更 + 自己创建的 Deployment 变更（级联触发）。
func (r *WebAppReconciler) SetupWithManager(mgr manager.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&webappv1.WebApp{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}

func main() {
	log.SetLogger(zap.New(zap.UseDevMode(true)))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建 manager 失败: %v\n", err)
		os.Exit(1)
	}

	if err := (&WebAppReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()}).SetupWithManager(mgr); err != nil {
		fmt.Fprintf(os.Stderr, "注册控制器失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("webapp-operator 启动（先 apply CRD 和 WebApp 再运行）")
	if err := mgr.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "manager 退出: %v\n", err)
		os.Exit(1)
	}
}
