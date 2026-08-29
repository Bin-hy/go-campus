// counter-controller：一个跑在真实集群上的最小控制器。
//
// 架构（与 K8s 官方 controller 完全一致）：
//   Informer（监听 ConfigMap）→ Workqueue（丢 key）→ Worker（Reconcile 收敛期望状态）
//
// 功能：监听 namespace 内带注解 counter.example.com/desired 的 ConfigMap，
//       确保同名 Deployment 的副本数等于注解值。
//
// 使用：见 README.md
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/workqueue"
)

const (
	annotationKey = "counter.example.com/desired"
)

// Controller 控制器的核心结构：client + informer + workqueue。
type Controller struct {
	client    kubernetes.Interface
	informer  cache.SharedIndexInformer
	workqueue workqueue.RateLimitingInterface
}

// NewController 组装 informer 与 workqueue，注册事件回调。
func NewController(client kubernetes.Interface, namespace string) *Controller {
	factory := informers.NewSharedInformerFactoryWithOptions(client, 30*time.Second,
		informers.WithNamespace(namespace))
	informer := factory.Core().V1().ConfigMaps().Informer()

	c := &Controller{
		client:    client,
		informer:  informer,
		workqueue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
	}

	// 事件回调只做一件事：把 key 丢进队列（namespace/name）
	_, _ = informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				c.workqueue.Add(key)
			}
		},
		UpdateFunc: func(old, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {
				c.workqueue.Add(key)
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				c.workqueue.Add(key)
			}
		},
	})

	return c
}

// Run 启动 informer 与 workers，阻塞直到 ctx 取消。
func (c *Controller) Run(ctx context.Context, workers int) error {
	defer c.workqueue.ShutDown()

	go c.informer.Run(ctx.Done())
	// 等待本地缓存同步完成，避免"缓存还没建好就开始 Reconcile"
	if !cache.WaitForCacheSync(ctx.Done(), c.informer.HasSynced) {
		return fmt.Errorf("等待 informer 缓存同步超时")
	}

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			wait.UntilWithContext(ctx, c.runWorker, time.Second)
		}()
	}
	<-ctx.Done()
	wg.Wait()
	return nil
}

func (c *Controller) runWorker(ctx context.Context) {
	for c.processNextItem(ctx) {
	}
}

func (c *Controller) processNextItem(ctx context.Context) bool {
	key, shutdown := c.workqueue.Get()
	if shutdown {
		return false
	}
	defer c.workqueue.Done(key)

	if err := c.reconcile(ctx, key.(string)); err != nil {
		// 失败：限速重试（指数退避）
		c.workqueue.AddRateLimited(key)
		fmt.Printf("reconcile %q 失败: %v\n", key, err)
		return true
	}
	c.workqueue.Forget(key)
	return true
}

// reconcile 核心逻辑：让 Deployment 副本数收敛到 ConfigMap 注解值。
// 期望状态：注解值；实际状态：Deployment.Spec.Replicas；有差距就 Update。
func (c *Controller) reconcile(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	cm, err := c.client.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		// ConfigMap 被删了 → 也把 Deployment 删掉（跟随删除）
		return c.client.AppsV1().Deployments(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	}
	if err != nil {
		return err
	}

	// 没有注解 → 忽略
	raw, ok := cm.Annotations[annotationKey]
	if !ok {
		return nil
	}
	desired, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("注解 %s=%q 不是合法数字", annotationKey, raw)
	}

	deploy, err := c.client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		// Deployment 不存在 → 创建
		_, err = c.client.AppsV1().Deployments(namespace).Create(ctx,
			appsv1Deployment(namespace, name, int32(desired)), metav1.CreateOptions{})
		return err
	}
	if err != nil {
		return err
	}

	// 副本数一致 → 无需动作（收敛完成）
	if deploy.Spec.Replicas != nil && int(*deploy.Spec.Replicas) == desired {
		return nil
	}
	deploy.Spec.Replicas = int32Ptr(int32(desired))
	_, err = c.client.AppsV1().Deployments(namespace).Update(ctx, deploy, metav1.UpdateOptions{})
	return err
}

// buildClient 从 kubeconfig 构建 clientset。
func buildClient() (*kubernetes.Clientset, error) {
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

func main() {
	client, err := buildClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "构建客户端失败: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() { <-sig; cancel() }()

	ns := "default"
	if len(os.Args) > 1 {
		ns = os.Args[1]
	}

	c := NewController(client, ns)
	fmt.Printf("counter-controller 启动，监听 namespace=%s 的 ConfigMap 注解 %s\n", ns, annotationKey)
	if err := c.Run(ctx, 2); err != nil {
		fmt.Fprintf(os.Stderr, "控制器退出: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("控制器已停止")
}

var _ = corev1.Pod{}
