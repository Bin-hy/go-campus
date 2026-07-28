package type_assert

import "fmt"

// Shape 图形接口
type Shape interface {
	Area() float64
	Perimeter() float64
}

// Circle 圆形
type Circle struct{ Radius float64 }

// Rectangle 矩形
type Rectangle struct{ Width, Height float64 }

// Triangle 三角形
type Triangle struct{ A, B, C float64 } // 三条边

// Describe 根据 Shape 的动态类型，返回描述字符串
// Circle: "Circle with radius X"
// Rectangle: "Rectangle WxH"
// Triangle: "Triangle with sides A, B, C"
// 其他: "Unknown shape"
func Describe(s Shape) string {
	// TODO: 使用 type switch 实现
	panic("not implemented")
}

// TotalArea 计算多个图形的总面积
func TotalArea(shapes []Shape) float64 {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// FilterByType 筛选指定类型的图形
// typeName: "circle", "rectangle", "triangle"
func FilterByType(shapes []Shape, typeName string) []Shape {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// --- 请实现以下方法 ---

func (c Circle) Area() float64 {
	// TODO
	panic("not implemented")
}

func (c Circle) Perimeter() float64 {
	// TODO
	panic("not implemented")
}

func (r Rectangle) Area() float64 {
	// TODO
	panic("not implemented")
}

func (r Rectangle) Perimeter() float64 {
	// TODO
	panic("not implemented")
}

// Triangle 使用海伦公式计算面积
func (t Triangle) Area() float64 {
	// TODO
	panic("not implemented")
}

func (t Triangle) Perimeter() float64 {
	// TODO
	panic("not implemented")
}

// Stringer 实现（用于调试）
func (c Circle) String() string    { return fmt.Sprintf("Circle(r=%.1f)", c.Radius) }
func (r Rectangle) String() string { return fmt.Sprintf("Rect(%.1fx%.1f)", r.Width, r.Height) }
func (t Triangle) String() string  { return fmt.Sprintf("Tri(%.1f,%.1f,%.1f)", t.A, t.B, t.C) }
