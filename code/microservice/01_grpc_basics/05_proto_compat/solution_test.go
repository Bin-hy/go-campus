package proto_compat

import (
	"bytes"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
)

func TestWireRoundTrip(t *testing.T) {
	b := EncodeUser(42, "alice", nil)
	id, name, unknown, err := DecodeUser(b)
	if err != nil {
		t.Fatal(err)
	}
	if id != 42 || name != "alice" || len(unknown) != 0 {
		t.Fatalf("got id=%d name=%q unknown=%x", id, name, unknown)
	}
}

func TestOldDecoderPreservesNewFields(t *testing.T) {
	// 新版本新增字段 3（例如 email）和字段 4（例如 version）。
	extra := map[int32][]byte{
		3: []byte("alice@campus.dev"),
		4: protowire.AppendVarint(nil, 7),
	}
	wire := EncodeUser(42, "alice", extra)

	// 旧版本只认识 1/2，但也必须原样保留 3/4。
	id, name, unknown, err := DecodeUser(wire)
	if err != nil {
		t.Fatal(err)
	}
	if id != 42 || name != "alice" {
		t.Fatalf("旧字段解析错误: %d %q", id, name)
	}
	if len(unknown) == 0 {
		t.Fatal("未知字段必须被保留，否则网关转发会丢字段")
	}

	// 模拟"旧代码重新编码"：已知字段 + 未知字段原样带回。
	var re []byte
	re = protowire.AppendTag(re, 1, protowire.VarintType)
	re = protowire.AppendVarint(re, uint64(id))
	re = protowire.AppendTag(re, 2, protowire.BytesType)
	re = protowire.AppendString(re, name)
	re = append(re, unknown...)

	if !bytes.Equal(collectionsOfFields(wire), collectionsOfFields(re)) {
		t.Fatalf("往返后字段集合应一致:\n wire=%x\n re  =%x", wire, re)
	}
	t.Logf("未知字段保留 OK：%x", unknown)
}

// collectionsOfFields 把编码按字段排序后比较（map 遍历顺序不固定）。
func collectionsOfFields(b []byte) []byte {
	var fields [][]byte
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			panic(n)
		}
		start := 0
		vn := protowire.ConsumeFieldValue(num, typ, b[n:])
		if vn < 0 {
			panic(vn)
		}
		fields = append(fields, b[start:start+n+vn])
		b = b[n+vn:]
	}
	// 冒泡排序，保持零依赖。
	for i := 0; i < len(fields); i++ {
		for j := i + 1; j < len(fields); j++ {
			if bytes.Compare(fields[j], fields[i]) < 0 {
				fields[i], fields[j] = fields[j], fields[i]
			}
		}
	}
	var out []byte
	for _, f := range fields {
		out = append(out, f...)
	}
	return out
}

func TestCompatibilityRules(t *testing.T) {
	base := Schema{Fields: []Field{
		{Number: 1, Name: "id", Kind: KindVarint},
		{Number: 2, Name: "name", Kind: KindBytes},
		{Number: 3, Name: "email", Kind: KindBytes},
	}}

	t.Run("新增字段兼容", func(t *testing.T) {
		next := base
		next.Fields = append(append([]Field{}, base.Fields...), Field{Number: 4, Name: "version", Kind: KindVarint})
		if issues := CheckCompatibility(base, next); len(issues) != 0 {
			t.Fatalf("新增字段应兼容: %v", issues)
		}
	})

	t.Run("删除字段未 reserved", func(t *testing.T) {
		next := Schema{Fields: []Field{base.Fields[0], base.Fields[1]}}
		issues := CheckCompatibility(base, next)
		if len(issues) == 0 || !strings.Contains(issues[0].String(), "reserved") {
			t.Fatalf("应提示未 reserved: %v", issues)
		}
	})

	t.Run("删除并 reserved 兼容", func(t *testing.T) {
		next := Schema{Fields: []Field{base.Fields[0], base.Fields[1]}, Reserved: []int32{3}}
		if issues := CheckCompatibility(base, next); len(issues) != 0 {
			t.Fatalf("reserved 后应兼容: %v", issues)
		}
	})

	t.Run("改线类型不兼容", func(t *testing.T) {
		next := Schema{Fields: []Field{
			{Number: 1, Name: "id", Kind: KindVarint},
			{Number: 2, Name: "name", Kind: KindFixed64}, // string → fixed64
			{Number: 3, Name: "email", Kind: KindBytes},
		}}
		issues := CheckCompatibility(base, next)
		if len(issues) == 0 {
			t.Fatal("改线类型应报不兼容")
		}
	})

	t.Run("varint 家族互换兼容", func(t *testing.T) {
		// int32 → int64 在线上都是 varint，属于兼容演进。
		next := Schema{Fields: []Field{base.Fields[0], base.Fields[1], base.Fields[2]}}
		next.Fields[0].Kind = KindVarint
		if issues := CheckCompatibility(base, next); len(issues) != 0 {
			t.Fatalf("同类线型应兼容: %v", issues)
		}
	})
}
