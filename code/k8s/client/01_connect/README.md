# 练习 1：连接集群并列出 Pod

## 考点
- kubeconfig / context 的概念（见教程第 01 章）
- client-go 的 clientset 构建：`clientcmd` + `kubernetes.NewForConfig`
- typed client 的 List 操作与 namespace 过滤

## 题目
1. `BuildClient`：从 kubeconfig 构建 clientset（支持 `KUBECONFIG` 环境变量，默认 `~/.kube/config`）。
2. `ListPodNames`：列出指定 namespace 下所有 Pod 的名字。

## 运行测试（无需集群）

```bash
cd code/k8s/client/01_connect
go test -v
```

## 真集群实操（连你的 minikube）

```bash
cd code/k8s/client/01_connect
go run .            # 列出 default namespace 的 Pod
go run . kube-system  # 列出 kube-system 的 Pod（控制面组件）
```

预期输出：kube-system 下能看到 coredns / etcd / kube-apiserver 等 6 个控制面 Pod。

## 提示
- `clientcmd.RecommendedHomeFile` 就是 `~/.kube/config`
- 先填 `solution.go`，测试通过后再对照 `answer/answer.go`
