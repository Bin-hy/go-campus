# 桶策略：最小权限的匿名只读前缀（安全）

## 难度：⭐⭐⭐ 较难

## 考点
- Bucket Policy（桶策略）：一段 S3 标准的 JSON，声明「谁、能对哪些资源、做什么操作」
- **最小权限原则**：只把 `public/` 前缀开放为匿名只读，`private/` 前缀绝不外泄
- `Principal: {"AWS": ["*"]}` 表示匿名（任何人）；`Resource` 用 ARN + 前缀通配精确圈定范围
- 权限验证要做**正向 + 反向**双向断言：该通的通，该拒的必须拒

## 环境准备

```bash
cd code/backend && docker compose up -d minio
```

## 题目描述

剪辑产品里，成片封面图（`public/`）要能被 CDN 匿名拉取，而用户的原始素材（`private/`）必须严格保密。做法是给桶设一条**只对 `public/` 前缀开放匿名只读**的策略。

请实现 `ApplyPublicReadPrefix`：给定 `bucket` 和 `prefix`（如 `public/`），构造并下发一条 S3 桶策略，效果是——**匿名用户只能 GET `<bucket>/<prefix>*` 下的对象**，其他一律拒绝。

测试会：
1. 放入 `public/logo.png` 与 `private/secret.mp4`；
2. 调你的函数对 `public/` 生效；
3. 用**裸 `http.Get`（无任何凭据）**访问 `public/logo.png` → 断言 200；
4. 同样方式访问 `private/secret.mp4` → 断言非 200（被拒），证明最小权限落地。

## 函数签名

```go
func ApplyPublicReadPrefix(ctx context.Context, client *minio.Client, bucket, prefix string) error
```

## 提示

1. 策略是一段 JSON 字符串，`Version` 固定 `"2012-10-17"`
2. 单条 `Statement`：`Effect: "Allow"`、`Principal: {"AWS":["*"]}`、`Action: ["s3:GetObject"]`
3. `Resource` 写成 `arn:aws:s3:::<bucket>/<prefix>*`——末尾的 `*` 让前缀下所有对象命中
4. 用 `fmt.Sprintf` 拼 JSON，再 `client.SetBucketPolicy(ctx, bucket, policy)`
5. 匿名访问的直链形如 `http://127.0.0.1:9000/<bucket>/<object>`

## 运行测试

```bash
cd code/backend/09-object-storage/03_bucket_policy && go test -v
```
