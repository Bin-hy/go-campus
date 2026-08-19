# SSE 流式客户端（解析 text/event-stream）

## 难度：⭐⭐ 中等

## 考点
- 流式解析：`data:` 行、空行分隔事件（`\n\n`）
- 多行 `data:` 合并、`[DONE]` 结束标记
- context 取消（断线 / 超时）

## 题目描述

实现 `ReadSSE`：从 `io.Reader` 逐事件解析 SSE 流，每条事件的内容（`data:` 行的值）回调给 `onData`。

要求：
1. 事件以空行（`\n\n`）分隔；一个事件可含多条 `data:` 行（用 `\n` 合并）
2. 遇到 `data: [DONE]` 视为流结束，返回 nil（不再回调）
3. `onData` 返回 error 时立即返回该 error
4. `ctx` 已取消时返回 `ctx.Err()`

## 函数签名

```go
func ReadSSE(ctx context.Context, r io.Reader, onData func(string) error) error
```

## 示例

```go
r := strings.NewReader("data: progress:30\n\ndata: progress:60\n\n")
ReadSSE(ctx, r, func(d string) error { fmt.Println(d); return nil })
// 输出：
// progress:30
// progress:60
```

## 提示
1. 用 `bufio.Scanner` 按行读；空行 = 事件结束
2. `strings.TrimPrefix(line, "data:")` 后 `strings.TrimSpace`
3. 在每个事件边界检查 `ctx.Done()`
4. 延伸：断线续传要靠服务端发 `id:` + 客户端重连带 `Last-Event-ID`（本练习可先忽略）

## 验收
- [ ] 单行 / 多行 `data:` 事件都能正确解析
- [ ] `[DONE]` 后停止；ctx 取消立即返回
