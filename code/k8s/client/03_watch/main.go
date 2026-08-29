package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

// go run . [namespace]
func main() {
	namespace := "default"
	if len(os.Args) > 1 {
		namespace = os.Args[1]
	}

	client, err := BuildClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "构建客户端失败: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 监听 Ctrl+C
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() { <-sig; cancel() }()

	// 构建 informer，并注册事件回调
	informer, err := BuildInformer(client, namespace)
	if err != nil {
		fmt.Fprintf(os.Stderr, "构建 informer 失败: %v\n", err)
		os.Exit(1)
	}

	_, _ = informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			fmt.Printf("[%s] Added    %s/%s\n", time.Now().Format("15:04:05"), pod.Namespace, pod.Name)
		},
		UpdateFunc: func(old, new interface{}) {
			pod := new.(*corev1.Pod)
			fmt.Printf("[%s] Modified %s/%s phase=%s\n", time.Now().Format("15:04:05"), pod.Namespace, pod.Name, pod.Status.Phase)
		},
		DeleteFunc: func(obj interface{}) {
			pod, ok := obj.(*corev1.Pod)
			if !ok {
				if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
					pod, _ = tombstone.Obj.(*corev1.Pod)
				}
			}
			if pod != nil {
				fmt.Printf("[%s] Deleted  %s/%s\n", time.Now().Format("15:04:05"), pod.Namespace, pod.Name)
			}
		},
	})

	fmt.Printf("监听 namespace=%s 的 Pod 事件，Ctrl+C 退出\n", namespace)
	informer.Run(ctx.Done())
	_ = metav1.ObjectMeta{}
}
