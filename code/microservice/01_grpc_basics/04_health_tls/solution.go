package health_tls

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// CertBundle 一套测试用 PKI：CA + 服务端证书 + 客户端证书。
type CertBundle struct {
	CAPEM         []byte
	ServerCertPEM []byte
	ServerKeyPEM  []byte
	ClientCertPEM []byte
	ClientKeyPEM  []byte
}

// GenerateCerts 生成自签名 CA，并签发服务端/客户端证书。
//
// 提示：用 crypto/ecdsa + crypto/x509 + encoding/pem；
// CA 需要 IsCA=true 与 KeyUsageCertSign；
// 服务端证书 SAN 至少包含 localhost 与 127.0.0.1；
// 客户端证书 ExtKeyUsage 用 ClientAuth。
func GenerateCerts() (*CertBundle, error) {
	// TODO: 实现你的代码
	panic("not implemented")
}

// ServerCreds 服务端 TLS；clientCAPEM 非空时开启 mTLS（RequireAndVerifyClientCert）。
func ServerCreds(certPEM, keyPEM, clientCAPEM []byte) (credentials.TransportCredentials, error) {
	// TODO: 实现你的代码
	panic("not implemented")
}

// ClientCreds 客户端 TLS；clientCertPEM 非空时携带客户端证书（配合 mTLS）。
func ClientCreds(caPEM, clientCertPEM, clientKeyPEM []byte, serverName string) (credentials.TransportCredentials, error) {
	// TODO: 实现你的代码
	panic("not implemented")
}

// CheckHealth 调标准 gRPC Health Checking 协议。
func CheckHealth(ctx context.Context, conn *grpc.ClientConn, service string) error {
	// TODO: 实现你的代码
	panic("not implemented")
}
