# Lab：K8s 部署（Deployment / Service / HPA / PDB）

> 教程章节：15 K8s 部署、探针、滚动发布与 HPA

## 目录

| 文件 | 内容 |
| ---- | ---- |
| `10-uservice.yaml` | Namespace + Deployment（3 副本、三种探针、preStop、资源限制） |
| `20-service.yaml` | ClusterIP + Headless Service（客户端 LB 用） |
| `30-hpa-pdb.yaml` | HPA（CPU 60%，2~8 副本）+ PodDisruptionBudget |
| `40-gateway.yaml` | 网关 Deployment + Service（DNS 寻址 Headless） |

## 构建镜像

minikube 下最省事：直接构建进 minikube 的 Docker 环境。

```bash
# 在 code/microservice 目录
eval $(minikube docker-env)

docker build -f lab/uservice/Dockerfile -t gocampus/uservice:dev .
docker build -f lab/gateway/Dockerfile   -t gocampus/gateway:dev .

kubectl apply -f lab/k8s/
kubectl -n microservice-demo get pods,svc,hpa -w
```

## 验证清单

```bash
# 1. 三个实例都 Ready
kubectl -n microservice-demo get pods -l app=uservice

# 2. 通过普通 Service 访问
kubectl -n microservice-demo run curl --rm -it --image=curlimages/curl -- \
  curl -s http://gateway.microservice-demo/user?id=1   # 网关端口 80

# 3. 滚动发布不中断
kubectl -n microservice-demo set image deploy/uservice uservice=gocampus/uservice:v2
kubectl -n microservice-demo rollout status deploy/uservice

# 4. 优雅停机：删除一个 Pod，观察日志先 NOT_SERVING 再退出
kubectl -n microservice-demo delete pod -l app=uservice --wait=false

# 5. HPA：压测后看副本数变化（需 metrics-server）
kubectl -n microservice-demo get hpa -w
```

## 三个容易踩的坑

1. **readiness 失败 ≠ 重启**：只会摘流量。把"依赖故障"写成 liveness 会导致所有实例一起重启，雪崩。
2. **preStop sleep 是必要的**：Endpoint 摘除是异步的，进程立刻退出会丢掉刚被转发的请求。
3. **HPA 需要 requests**：没有 `resources.requests.cpu`，HPA 无法计算利用率，会一直是 `<unknown>`。
