package health_tls

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// 参考答案 / 可运行实现。

// CertBundle 一套测试用 PKI：CA + 服务端证书 + 客户端证书。
type CertBundle struct {
	CAPEM         []byte
	ServerCertPEM []byte
	ServerKeyPEM  []byte
	ClientCertPEM []byte
	ClientKeyPEM  []byte
}

// GenerateCerts 生成自签名 CA，并签发服务端/客户端证书。
// 生产环境用公司 CA / cert-manager，这里是为了让实验零外部依赖可复现。
func GenerateCerts() (*CertBundle, error) {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "campus-test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, err
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		return nil, err
	}

	sign := func(serial int64, cn string, dns []string, ips []net.IP, client bool) (certPEM, keyPEM []byte, err error) {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, nil, err
		}
		usage := []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
		if client {
			usage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
		}
		tmpl := &x509.Certificate{
			SerialNumber: big.NewInt(serial),
			Subject:      pkix.Name{CommonName: cn},
			NotBefore:    time.Now().Add(-time.Hour),
			NotAfter:     time.Now().Add(24 * time.Hour),
			KeyUsage:     x509.KeyUsageDigitalSignature,
			ExtKeyUsage:  usage,
			DNSNames:     dns,
			IPAddresses:  ips,
		}
		der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
		if err != nil {
			return nil, nil, err
		}
		keyDER, err := x509.MarshalPKCS8PrivateKey(key)
		if err != nil {
			return nil, nil, err
		}
		return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
			pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), nil
	}

	serverCert, serverKey, err := sign(2, "uservice", []string{"localhost", "uservice.local"}, []net.IP{net.ParseIP("127.0.0.1")}, false)
	if err != nil {
		return nil, err
	}
	clientCert, clientKey, err := sign(3, "gateway", nil, nil, true)
	if err != nil {
		return nil, err
	}
	return &CertBundle{
		CAPEM:         pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}),
		ServerCertPEM: serverCert,
		ServerKeyPEM:  serverKey,
		ClientCertPEM: clientCert,
		ClientKeyPEM:  clientKey,
	}, nil
}

// ServerCreds 服务端 TLS；clientCAPEM 非空时开启 mTLS（校验客户端证书）。
func ServerCreds(certPEM, keyPEM, clientCAPEM []byte) (credentials.TransportCredentials, error) {
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	cfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
	if len(clientCAPEM) > 0 {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(clientCAPEM) {
			return nil, errInvalidPEM
		}
		cfg.ClientCAs = pool
		cfg.ClientAuth = tls.RequireAndVerifyClientCert // mTLS：双向校验
	}
	return credentials.NewTLS(cfg), nil
}

// ClientCreds 客户端 TLS；clientCertPEM 非空时携带客户端证书（配合 mTLS）。
func ClientCreds(caPEM, clientCertPEM, clientKeyPEM []byte, serverName string) (credentials.TransportCredentials, error) {
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, errInvalidPEM
	}
	cfg := &tls.Config{
		RootCAs:    pool,
		ServerName: serverName,
		MinVersion: tls.VersionTLS12,
	}
	if len(clientCertPEM) > 0 {
		cert, err := tls.X509KeyPair(clientCertPEM, clientKeyPEM)
		if err != nil {
			return nil, err
		}
		cfg.Certificates = []tls.Certificate{cert}
	}
	return credentials.NewTLS(cfg), nil
}

// CheckHealth 调标准 gRPC Health Checking 协议。
func CheckHealth(ctx context.Context, conn *grpc.ClientConn, service string) error {
	_, err := healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{Service: service})
	return err
}

type constErr string

func (e constErr) Error() string { return string(e) }

const errInvalidPEM = constErr("invalid PEM")
