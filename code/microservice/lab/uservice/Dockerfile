# 构建：在 code/microservice 目录执行
#   docker build -f lab/uservice/Dockerfile -t gocampus/uservice:dev .
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/uservice ./lab/uservice

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/uservice /uservice
EXPOSE 9001 9101
ENTRYPOINT ["/uservice"]
