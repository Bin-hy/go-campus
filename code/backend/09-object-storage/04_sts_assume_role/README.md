# STS 临时凭证：AssumeRole 换 scoped 直传凭证（安全）

## 难度：⭐⭐⭐ 较难

## 考点
- **STS AssumeRole**：后端持长期密钥，向 STS 换回临时三元组（AK/SK + SessionToken），交给客户端
- **会话策略取交集**：临时凭证的实际权限 = 签发身份权限 ∩ 内联会话策略，只会更窄、绝不会更宽
- 会话策略是 **IAM 风格、无 `Principal`**（区别于第三篇桶策略的 `Principal:{"AWS":["*"]}`）
- **客户端直传的最小权限**：把凭证死死圈在 `clips/<userID>/*`，用户 A 碰不到用户 B 的素材
- 验证要做**正向 + 反向**双向断言：授权前缀内可读写；越权前缀必须 `AccessDenied`

## 环境准备

```bash
cd code/backend && docker compose up -d minio
```

## 题目描述

剪映/CapCut 客户端要把海量素材**直传**到对象存储。绝不能把后端的长期 AK/SK 下发给客户端（泄露=全盘沦陷）；也不适合每个对象都单独签一个预签名 URL（客户端要做多次、多对象操作）。生产做法是：**后端用长期密钥向 STS AssumeRole，附一条内联会话策略把权限收窄到当前用户的前缀，换回一组短过期的临时三元组交给客户端**。

请实现 `AssumeScopedRole`：给定长期密钥与 `bucket`、`allowedPrefix`（如 `clips/alice/`），构造内联会话策略（只放行该前缀下的 `s3:GetObject` + `s3:PutObject`），向 STS 换回临时凭证，返回一个用该临时凭证构建的 `*minio.Client`。

测试会：
1. 后端(admin)建桶并播种 `clips/alice/take1.mp4` 与 `clips/bob/take1.mp4`；
2. 调你的函数换取圈定在 `clips/alice/` 的 scoped client；
3. **正向**：scoped client 读回 `clips/alice/take1.mp4`（内容一致）、写入 `clips/alice/take2.mp4` → 都成功；
4. **反向**：scoped client 读/写 `clips/bob/*` → 断言 `AccessDenied`，证明 scope 落地。

## 函数签名

```go
func AssumeScopedRole(endpoint, accessKey, secretKey, bucket, allowedPrefix string, ttl time.Duration) (*minio.Client, error)
```

## 提示

1. 会话策略是一段 JSON：`Version` 固定 `"2012-10-17"`，单条 `Statement` 为 `Effect:"Allow"`、`Action:["s3:GetObject","s3:PutObject"]`、`Resource:["arn:aws:s3:::<bucket>/<allowedPrefix>*"]`——**注意没有 `Principal`**
2. `credentials.NewSTSAssumeRole(stsEndpoint, credentials.STSAssumeRoleOptions{AccessKey, SecretKey, Policy, DurationSeconds})`，其中 `stsEndpoint` 要**带 scheme**（`"http://" + endpoint`），返回 `*credentials.Credentials`
3. 把临时凭证喂给 `minio.New(endpoint, &minio.Options{Creds: stsCreds, Secure: false})`——这里的 `endpoint` **不带 scheme**
4. 踩坑：minio-go v7.3.0 的 `DurationSeconds` 只在 `>3600` 时生效，否则强制回落到 3600s（1h），所以传 `time.Hour` 及以下拿到的都是 1 小时
5. 踩坑：`GetObject` 是**惰性**的——`GetObject()` 本身不报错，越权错误要到首次 `Stat()`/`Read()` 才暴露；`PutObject` 是即时的，越权直接返回错误
6. 判定越权：`minio.ToErrorResponse(err).Code == "AccessDenied"`

## 运行测试

```bash
cd code/backend/09-object-storage/04_sts_assume_role && go test -v
```
