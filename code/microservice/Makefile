.PHONY: proto build test race vet run-user run-gateway clean

proto:
	protoc -I api --go_out=. --go_opt=module=gocampus/microservice \
	       --go-grpc_out=. --go-grpc_opt=module=gocampus/microservice \
	       api/user/v1/user.proto

build:
	go build ./...

test:
	go test ./...

race:
	go test -race ./02_governance/... ./03_messaging/... ./04_observability/...

vet:
	go vet ./...

# 三个实例请分别开终端执行；这里只起一个方便快速验证
run-user:
	go run ./lab/uservice -addr :9001 -instance user-1 -metrics-addr :9101

run-gateway:
	go run ./lab/gateway -targets 127.0.0.1:9001,127.0.0.1:9002,127.0.0.1:9003

clean:
	go clean -testcache
