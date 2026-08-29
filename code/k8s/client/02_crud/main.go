package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"
)

// go run . create|list|scale <n>|delete
func main() {
	if len(os.Args) < 2 {
		fmt.Println("用法: go run . <create|list|scale N|delete>")
		os.Exit(1)
	}

	client, err := BuildClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "构建客户端失败: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	ns := "default"
	name := "web"

	switch os.Args[1] {
	case "create":
		d, err := CreateDeployment(ctx, client, ns, name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "创建失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("已创建 Deployment %s，副本数 %d\n", d.Name, *d.Spec.Replicas)
	case "list":
		names, err := ListDeployments(ctx, client, ns)
		if err != nil {
			fmt.Fprintf(os.Stderr, "列出失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("namespace=%s 的 Deployment: %v\n", ns, names)
	case "scale":
		if len(os.Args) < 3 {
			fmt.Println("需要副本数: go run . scale 5")
			os.Exit(1)
		}
		n, err := strconv.Atoi(os.Args[2])
		if err != nil {
			fmt.Fprintf(os.Stderr, "副本数解析失败: %v\n", err)
			os.Exit(1)
		}
		d, err := ScaleDeployment(ctx, client, ns, name, int32(n))
		if err != nil {
			fmt.Fprintf(os.Stderr, "扩缩容失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("已扩缩容到 %d 副本\n", *d.Spec.Replicas)
	case "delete":
		if err := DeleteDeployment(ctx, client, ns, name); err != nil {
			fmt.Fprintf(os.Stderr, "删除失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("已删除")
	default:
		fmt.Println("未知命令: ", os.Args[1])
		os.Exit(1)
	}
}
