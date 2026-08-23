package sts_assume_role

import (
	"fmt"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// AssumeScopedRole 参考答案：向 STS 换取一组「只在指定前缀内可读写、会过期」的
// 临时凭证，并用它构建一个受限的 minio.Client。
//
// 这是客户端直传素材的生产范式：后端手握长期密钥（accessKey/secretKey），
// 但绝不把长期密钥下发给客户端——而是 AssumeRole 换一组临时三元组
// （AK/SK/SessionToken）交给客户端，权限被内联会话策略死死圈在
// bucket/allowedPrefix* 之内，且几十分钟就过期。剪映场景里，
// allowedPrefix 取 "clips/<userID>/"，用户 A 的凭证碰不到用户 B 的素材。
//
// 三个关键点（面试高频、也是本题考点）：
//  1. 会话策略是 IAM 风格，没有 Principal 字段——它描述「这组临时凭证能做什么」，
//     而不是「谁能访问这个桶」（后者是第三篇的资源型桶策略，带 Principal）。
//  2. 临时凭证的实际权限 = 签发身份的权限 ∩ 这条内联会话策略，只会更窄、绝不会更宽。
//     用 root 签发时，root 是全权限，交集就等于这条内联策略本身。
//  3. STS 端点是「带 scheme 的完整 URL」（http://host:port），和 S3 端点同主机；
//     而 minio.New 只吃不带 scheme 的 host:port，由 Secure 决定走 http 还是 https。
func AssumeScopedRole(endpoint, accessKey, secretKey, bucket, allowedPrefix string, ttl time.Duration) (*minio.Client, error) {
	// 1) 内联会话策略：只放行 bucket/allowedPrefix* 上的读(GetObject)与写(PutObject)，
	//    其余一律不授权（默认拒绝）。注意没有 Principal——这是会话策略与桶策略的关键区别。
	sessionPolicy := fmt.Sprintf(`{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": ["s3:GetObject", "s3:PutObject"],
      "Resource": ["arn:aws:s3:::%s/%s*"]
    }
  ]
}`, bucket, allowedPrefix)

	// 2) 向 STS 端点发起 AssumeRole，拿回「带边界、会过期」的临时三元组。
	//    stsEndpoint 必须带 scheme（url.Parse 解析后取 host），故补上 "http://"。
	//    DurationSeconds 注意：minio-go v7.3.0 只在 >3600 时生效，否则强制回落到 3600(1h)，
	//    所以这里传入 ttl 换算的秒数，实际拿到的过期时间下限是 1 小时。
	stsCreds, err := credentials.NewSTSAssumeRole("http://"+endpoint, credentials.STSAssumeRoleOptions{
		AccessKey:       accessKey,
		SecretKey:       secretKey,
		Policy:          sessionPolicy,
		DurationSeconds: int(ttl.Seconds()),
		RoleSessionName: "capcut-clip-uploader", // 便于服务端审计溯源：这组临时凭证是谁、为何签发
	})
	if err != nil {
		return nil, fmt.Errorf("STS AssumeRole 失败: %w", err)
	}

	// 3) 用临时凭证构建一个受限 client。它手上只有短命的临时三元组，
	//    越权访问（碰 allowedPrefix 之外的对象）会被服务端直接判 AccessDenied。
	//    线上务必把 Secure 改成 true 走 TLS，切勿明文传临时凭证。
	scoped, err := minio.New(endpoint, &minio.Options{
		Creds:  stsCreds,
		Secure: false, // 本地 dev；线上 = true(TLS)
	})
	if err != nil {
		return nil, fmt.Errorf("用临时凭证构建 client 失败: %w", err)
	}
	return scoped, nil
}
