package proto_compat

import "fmt"

// Kind 是 protobuf 的线类型（wire type）。
type Kind int

const (
	KindVarint Kind = iota // int32/int64/uint/bool/enum
	KindBytes              // string/bytes/message
	KindFixed32
	KindFixed64
)

// Field 是 schema 里的一个字段。
type Field struct {
	Number int32
	Name   string
	Kind   Kind
}

// Schema 代表一个 .proto message 的字段集合。
type Schema struct {
	Fields   []Field
	Reserved []int32 // 已删除字段的编号必须 reserved
}

// Issue 是一条兼容性问题。
type Issue struct {
	Field   int32
	Message string
}

func (i Issue) String() string { return fmt.Sprintf("field %d: %s", i.Field, i.Message) }

// CheckCompatibility 比较新旧 schema，返回兼容性问题列表。
//
// 规则：
//  1. 删除的字段必须在 new.Reserved 里，否则报问题；
//  2. 同一编号的线类型跨族变化报问题（varint 家族内部、string/bytes 之间兼容）；
//  3. 新字段不能占用 reserved 编号；
//  4. 字段改名线兼容，但提示影响 JSON/反射。
func CheckCompatibility(old, new Schema) []Issue {
	// TODO: 实现你的代码
	panic("not implemented")
}

// EncodeUser 手写 User 的线上编码：id=1(varint), name=2(bytes)。
// extra 用来模拟新版本追加的字段（编号 → 原始字节），验证旧代码能否原样保留。
//
// 提示：google.golang.org/protobuf/encoding/protowire 的 AppendTag/AppendVarint/AppendString。
func EncodeUser(id int64, name string, extra map[int32][]byte) []byte {
	// TODO: 实现你的代码
	panic("not implemented")
}

// DecodeUser 解析已知字段 1/2，并把未知字段（tag + value）原样保存。
func DecodeUser(b []byte) (id int64, name string, unknown []byte, err error) {
	// TODO: 实现你的代码
	panic("not implemented")
}
