// L01 · CPU 打满：反射 + JSON 序列化 + 字符串拼接
//
// 业务场景（模拟剪映「一键成片」的素材描述汇总）：
// 每个请求把 n 条素材记录拼成一段描述文本并计算总分。
//
// bug 模式用四种典型的 CPU 浪费写法：反射遍历字段、fmt.Sprintf 格式化、
// json.Marshal/Unmarshal 往返、`out += ...` 字符串累加（O(n²) 拷贝）。
// fix 模式改为字段直取 + strings.Builder + strconv 手写编码，完全不走反射。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"gocampus/perf/pprof-lab/internal/labkit"
)

var (
	addrFlag      = flag.String("addr", "127.0.0.1:18081", "业务监听地址")
	pprofAddrFlag = flag.String("pprof-addr", "127.0.0.1:19081", "pprof 管理监听地址（生产只绑内网）")
	fixFlag       = flag.Bool("fix", false, "使用修复实现（手写编码 / strings.Builder / 避免反射）")
)

const (
	defaultN = 2000  // /api/render 默认条数，约几十毫秒 CPU
	maxN     = 20000 // 上限，防止被一个请求打爆
)

var tags = []string{"cut", "beat", "transition", "filter", "subtitle", "bgm"}

// RenderItem 是「渲染结果」的一条记录，模拟一个素材片段。
type RenderItem struct {
	ID     int
	Title  string
	Tag    string
	Score  float64
	Frames int
	Weight float64
}

// buildItem 生成第 i 条记录，bug / fix 两条路径共用（保证对比公平）。
func buildItem(i int) RenderItem {
	return RenderItem{
		ID:     i,
		Title:  "clip-" + strconv.Itoa(i%997),
		Tag:    tags[i%len(tags)],
		Score:  float64(i%1000) / 10,
		Frames: 30 + i%120,
		Weight: float64(i%50) / 100,
	}
}

// renderBug 是问题实现：对每条记录做「反射 + 格式化 + JSON 往返 + 字符串累加」。
func renderBug(n int) (float64, string) {
	var out string
	var score float64
	for i := 0; i < n; i++ {
		it := buildItem(i)

		// ⚠️ 问题点 1：反射遍历结构体字段；Field(j).Interface() 会把值装箱到 interface{}，
		// 每次都要分配，而且是纯粹的运行时开销——字段在编译期就已知。
		v := reflect.ValueOf(it)
		t := v.Type()
		var item string
		for j := 0; j < v.NumField(); j++ {
			name := t.Field(j).Name
			// ⚠️ 问题点 2：fmt.Sprintf("%v") 内部再走一遍反射格式化，单次 100ns 量级。
			item += name + "=" + fmt.Sprintf("%v", v.Field(j).Interface()) + ";"
		}

		// ⚠️ 问题点 3：json.Marshal 本身走反射编码，紧接着又 Unmarshal 回 map[string]any，
		// 一次序列化 + 一次反序列化，只为了读出 Score 一个字段。
		if b, err := json.Marshal(it); err == nil {
			var m map[string]any
			if err := json.Unmarshal(b, &m); err == nil {
				if f, ok := m["Score"].(float64); ok {
					score += f
				}
			}
		}

		// ⚠️ 问题点 4：字符串累加每次都整体拷贝已有内容，n 条记录就是 O(n²) 的拷贝量
		// （n=2000、每条约 60 字节时，光拷贝就有 100MB 级别），
		// 堆上还留下大量很快变垃圾的中间字符串，GC 压力跟着上来。
		out += item + "|"
	}
	return score, out
}

// renderFix 是修复实现：字段直取 + 预分配 Builder + strconv 追加。
func renderFix(n int) (float64, string) {
	var sb strings.Builder
	// ✅ 修复 1：一次性预分配，避免 Builder 反复扩容和拷贝。
	sb.Grow(n * 64)

	var score float64
	for i := 0; i < n; i++ {
		it := buildItem(i)
		score += it.Score

		// ✅ 修复 2：字段直取，零反射；数字用 strconv 追加，不产生中间字符串。
		sb.WriteString("ID=")
		sb.WriteString(strconv.Itoa(it.ID))
		sb.WriteString(";Title=")
		sb.WriteString(it.Title)
		sb.WriteString(";Tag=")
		sb.WriteString(it.Tag)
		sb.WriteString(";Score=")
		appendFloat(&sb, it.Score)
		sb.WriteString(";Frames=")
		sb.WriteString(strconv.Itoa(it.Frames))
		sb.WriteString(";Weight=")
		appendFloat(&sb, it.Weight)
		sb.WriteString(";|")
	}
	// ✅ 修复 3：只做一次 String()，整个请求只在最后产生一个字符串。
	return score, sb.String()
}

// appendFloat 用 strconv 的低层接口把 float 直接追加进 Builder，避免分配临时字符串。
func appendFloat(sb *strings.Builder, f float64) {
	var buf [24]byte
	sb.Write(strconv.AppendFloat(buf[:0], f, 'f', 2, 64))
}

// render 是两个实现的统一入口：handler 与 benchmark 都通过它切换 bug / fix。
func render(n int, fix bool) (float64, string) {
	if fix {
		return renderFix(n)
	}
	return renderBug(n)
}

// newMux 构造业务路由（pprof 不在这里，它在 19081 上）。
func newMux(fix bool) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/render", func(w http.ResponseWriter, r *http.Request) {
		n := labkit.QueryInt(r, "n", defaultN, 1, maxN)
		start := time.Now()
		score, out := render(n, fix)
		elapsed := time.Since(start)
		labkit.WriteJSON(w, map[string]any{
			"mode":       labkit.ModeName(fix),
			"n":          n,
			"elapsed_ms": float64(elapsed.Microseconds()) / 1000,
			"out_bytes":  len(out),
			"score":      score,
		})
	})
	return mux
}

func main() {
	flag.Parse()
	labkit.Run(labkit.Config{
		BizAddr:   *addrFlag,
		PprofAddr: *pprofAddrFlag,
		Fix:       *fixFlag,
		Handler:   newMux(*fixFlag),
		BizNote:   "GET /api/render?n=2000",
		Problem:   "每次请求对 n 条记录做 反射 + fmt.Sprintf + json 往返 + 字符串累加，CPU 直接打满",
		FixNote:   "字段直取 + strings.Builder 预分配 + strconv 追加，热点从反射 / 格式化变成纯拷贝",
	})
}
