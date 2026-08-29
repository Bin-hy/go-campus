package main

import (
	"context"
	"fmt"
	"os"
	"time"
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	names, err := ListPodNames(ctx, client, namespace)
	if err != nil {
		fmt.Fprintf(os.Stderr, "列出 Pod 失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("namespace=%s 共有 %d 个 Pod:\n", namespace, len(names))
	for _, n := range names {
		fmt.Println(" -", n)
	}
}
