package proto_compat

import (
	"fmt"

	"google.golang.org/protobuf/encoding/protowire"
)

// 参考答案 / 可运行实现。
//
// 这题不依赖 protoc：用官方 protowire 直接读写二进制，亲手验证
// "字段编号才是线上契约，字段名只是给人看的"。

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
	Reserved []int32 // 已删除字段的编号必须 reserved，防止被误复用
}

// Issue 是一条兼容性问题。
type Issue struct {
	Field   int32
	Message string
}

func (i Issue) String() string { return fmt.Sprintf("field %d: %s", i.Field, i.Message) }

// CheckCompatibility 比较新旧 schema。
// 空返回表示向后兼容（新代码能读旧数据、旧代码能读新数据）。
func CheckCompatibility(old, new Schema) []Issue {
	var issues []Issue

	oldByNum := make(map[int32]Field, len(old.Fields))
	for _, f := range old.Fields {
		oldByNum[f.Number] = f
	}
	newByNum := make(map[int32]Field, len(new.Fields))
	for _, f := range new.Fields {
		newByNum[f.Number] = f
	}

	// 1. 删除字段必须 reserved；否则编号可能被后来的人复用。
	for _, f := range old.Fields {
		if _, ok := newByNum[f.Number]; ok {
			continue
		}
		if !contains(new.Reserved, f.Number) {
			issues = append(issues, Issue{f.Number, "字段已删除但没有 reserved，编号可能被误复用"})
		}
	}

	// 2. 同一编号的线类型必须兼容。
	// 注意：int32/int64/uint32/uint64/bool/enum 都编码成 varint，互相改动在线上是兼容的；
	// string 与 bytes 都是 length-delimited，也兼容；跨族改动则会乱码。
	for num, nf := range newByNum {
		of, ok := oldByNum[num]
		if !ok {
			continue
		}
		if of.Kind != nf.Kind {
			issues = append(issues, Issue{num, fmt.Sprintf("线类型从 %v 改为 %v，旧数据会解析失败", of.Kind, nf.Kind)})
		}
		if of.Name != nf.Name {
			// 改名不破坏线协议，但会破坏 JSON/反射与代码可读性，给一条提示。
			issues = append(issues, Issue{num, fmt.Sprintf("字段名从 %q 改为 %q（线兼容，但影响 JSON/反射）", of.Name, nf.Name)})
		}
	}

	// 3. 新字段不能占用 reserved 编号。
	for _, nf := range new.Fields {
		if contains(new.Reserved, nf.Number) {
			issues = append(issues, Issue{nf.Number, "新字段使用了 reserved 编号"})
		}
	}

	return issues
}

func contains(nums []int32, n int32) bool {
	for _, x := range nums {
		if x == n {
			return true
		}
	}
	return false
}

// EncodeUser 手写 User 的线上编码：id=1(varint), name=2(bytes)。
// extra 用来模拟"新版本新增的字段"，验证旧代码能否原样保留。
func EncodeUser(id int64, name string, extra map[int32][]byte) []byte {
	var b []byte
	b = protowire.AppendTag(b, 1, protowire.VarintType)
	b = protowire.AppendVarint(b, uint64(id))
	b = protowire.AppendTag(b, 2, protowire.BytesType)
	b = protowire.AppendString(b, name)
	for num, raw := range extra {
		b = protowire.AppendTag(b, protowire.Number(num), protowire.BytesType)
		b = protowire.AppendBytes(b, raw)
	}
	return b
}

// DecodeUser 解析已知字段 1/2，并把未知字段原样保存。
// 这是网关/代理"不丢字段"的关键：不认识也要保留，转发时原样带回。
func DecodeUser(b []byte) (id int64, name string, unknown []byte, err error) {
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return 0, "", nil, protowire.ParseError(n)
		}
		b = b[n:]
		switch {
		case num == 1 && typ == protowire.VarintType:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return 0, "", nil, protowire.ParseError(n)
			}
			id = int64(v)
			b = b[n:]
		case num == 2 && typ == protowire.BytesType:
			v, n := protowire.ConsumeString(b)
			if n < 0 {
				return 0, "", nil, protowire.ParseError(n)
			}
			name = v
			b = b[n:]
		default:
			// 未知字段：把整个 tag + value 原始字节收集起来。
			n := protowire.ConsumeFieldValue(num, typ, b)
			if n < 0 {
				return 0, "", nil, protowire.ParseError(n)
			}
			unknown = protowire.AppendTag(unknown, num, typ)
			unknown = append(unknown, b[:n]...)
			b = b[n:]
		}
	}
	return id, name, unknown, nil
}
