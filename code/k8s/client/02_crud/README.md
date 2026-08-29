# 练习 2：typed client 增删改查 Deployment

## 考点
- typed client（`client.AppsV1().Deployments(ns)`）
- Create / Get / Update / Delete / List 五个基本操作
- 指针字段 `*replicas` 的使用（`k8s.io/utils/ptr`）

## 题目
1. `CreateDeployment`：创建 2 副本 nginx Deployment（带 label selector）
2. `ScaleDeployment`：Get → 改 replicas → Update
3. `DeleteDeployment`：gracePeriod=0 立即删除
4. `ListDeployments`：列出名字

## 运行测试（无需集群，使用 fake clientset）

```bash
cd code/k8s/client/02_crud
go test -v
```

## 真集群实操

```bash
cd code/k8s/client/02_crud
go run . create    # 创建 web
go run . list      # 看到 web
go run . scale 5   # 扩到 5 副本
kubectl get deploy,rs,pods -l app=web   # 观察控制器自动补 Pod
go run . delete    # 删除
```

## 关键点
- `Create` 的 Deployment 必须带 `selector`，且 selector 要匹配 template 的 labels，否则 API Server 直接拒绝
- 更新要用**更新后的完整对象**调用 Update（client-go 不做乐观锁，生产多用 Patch）
