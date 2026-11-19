# Proto 兼容性：字段编号才是线上契约

## 难度：⭐⭐⭐ 困难

## 考点
- protobuf 的 wire format：tag = `(field_number << 3) | wire_type`
- "字段名随便改没事，字段编号不能动"
- 删除字段必须 `reserved`，否则编号会被后人复用
- 未知字段必须保留（代理/网关转发不能丢字段）
- 线类型兼容矩阵（varint 家族 / length-delimited 家族）
- `protoc` 的 breaking change 检测（`buf breaking` 思路）

## 题目描述

不依赖 protoc，用 `google.golang.org/protobuf/encoding/protowire` 手写：

1. `EncodeUser(id, name, extra)`：字段 1 为 varint，字段 2 为 string，`extra` 表示新版本追加的字段；
2. `DecodeUser(b) (id, name, unknown, err)`：只认识 1/2，但未知字段要原样收集；
3. `CheckCompatibility(old, new Schema) []Issue`：检查删除未 reserved、线类型变化、字段改名、占用 reserved 编号。

## 函数签名

```go
type Kind int
const ( KindVarint Kind = iota; KindBytes; KindFixed32; KindFixed64 )

type Field struct { Number int32; Name string; Kind Kind }
type Schema struct { Fields []Field; Reserved []int32 }
type Issue struct { Field int32; Message string }

func CheckCompatibility(old, new Schema) []Issue
func EncodeUser(id int64, name string, extra map[int32][]byte) []byte
func DecodeUser(b []byte) (id int64, name string, unknown []byte, err error)
```

## 提示

1. `protowire.AppendTag(b, num, protowire.VarintType)`；`AppendString` 自带长度前缀。
2. 解析循环：`ConsumeTag` → 已知字段用 `ConsumeVarint/ConsumeString`；未知字段用 `ConsumeFieldValue`，再把 **tag + value** 拼回去。
3. 判断兼容时别只看类型名：`int32/int64/uint32/uint64/bool/enum` 都是 varint；`string/bytes` 都是 length-delimited。
4. 测试里比较字段集合时，先按字节排序，因为 map 遍历顺序不固定。

## 验收

- [ ] 1/2 字段编码解码正确
- [ ] 新增字段 3/4 后，旧解码器能读出 1/2，且 `unknown` 非空；重新编码后字段集合完全一致
- [ ] 删除字段未 reserved → 报告问题
- [ ] string → fixed64 → 报告不兼容
- [ ] `go test ./01_grpc_basics/05_proto_compat`

## 运行

```bash
cd code/microservice
go test -v ./01_grpc_basics/05_proto_compat
```

## 面试追问

1. **为什么不能复用字段编号？** 旧客户端仍会按编号解析，新语义的字段会被当成老字段读取，产生"静默错误"而不是报错。
2. **新增字段为什么安全？** 旧代码不认识就跳过并保留；proto3 的默认值不编码，所以新字段默认值不会覆盖旧数据。
3. **`optional` 和 `oneof` 的兼容性？** proto3 `optional` 引入字段 presence，与普通标量线格式兼容；把字段移进/移出 `oneof` 也是兼容的（但 JSON 映射会变）。
4. **工程上怎么防？** CI 里跑 `buf breaking --against .git#branch=main`，或者维护 `reserved` 清单 + code review。
