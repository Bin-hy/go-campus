# SSE 流式服务端（text/event-stream）

## 难度：⭐⭐ 中等

## 考点
- SSE 协议格式（Content-Type、`data:` 行、空行分隔 `\n\n`）
- `http.Flusher` 流式刷新（不 Flush 客户端一条都收不到）
- 流式输出在 AI 场景的应用（任务进度推送、LLM 流式回答）

## 题目描述

实现 `SSEHandler`：返回一个 `http.HandlerFunc`，把 `messages` 逐条以 SSE 格式写出并 `Flush()`。

要求：
1. 响应头：`Content-Type: text/event-stream`、`Cache-Control: no-cache`、`Connection: keep-alive`
2. 每条消息输出 `data: <msg>\n\n`
3. 每条消息后调用 `Flush()`（响应器不支持 Flusher 时忽略）
4. 写完所有消息后正常返回（客户端读到 EOF 表示流结束）

## 函数签名

```go
func SSEHandler(messages []string) http.HandlerFunc
```

## 示例

```go
h := SSEHandler([]string{"progress:30", "progress:60", "done"})
// 客户端依次收到：
// data: progress:30
//
// data: progress:60
//
// data: done
//
```

## 提示
1. 用 `w.Header().Set(...)` 设置响应头
2. 通过 `w.(http.Flusher)` 断言获取 Flusher（旧实现可能不支持）
3. 思考：去掉 `Flush()` 会发生什么？（数据全部积压在缓冲区，连接关闭才一次性吐出）

## 验收
- [ ] `curl -N` 能看到每条 `data:` 事件
- [ ] 能讲出 Flush 的作用与去掉后的后果
