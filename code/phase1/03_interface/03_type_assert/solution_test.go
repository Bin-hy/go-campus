package type_assert

import (
	"math"
	"testing"
)

const epsilon = 1e-9

func floatEqual(a, b float64) bool {
	return math.Abs(a-b) < epsilon
}

func TestCircleArea(t *testing.T) {
	c := Circle{Radius: 5}
	expected := math.Pi * 25
	if !floatEqual(c.Area(), expected) {
		t.Errorf("Circle(5).Area() = %f，期望 %f", c.Area(), expected)
	}
}

func TestCirclePerimeter(t *testing.T) {
	c := Circle{Radius: 3}
	expected := 2 * math.Pi * 3
	if !floatEqual(c.Perimeter(), expected) {
		t.Errorf("Circle(3).Perimeter() = %f，期望 %f", c.Perimeter(), expected)
	}
}

func TestRectangleArea(t *testing.T) {
	r := Rectangle{Width: 4, Height: 6}
	if !floatEqual(r.Area(), 24) {
		t.Errorf("Rect(4x6).Area() = %f，期望 24", r.Area())
	}
}

func TestRectanglePerimeter(t *testing.T) {
	r := Rectangle{Width: 4, Height: 6}
	if !floatEqual(r.Perimeter(), 20) {
		t.Errorf("Rect(4x6).Perimeter() = %f，期望 20", r.Perimeter())
	}
}

func TestTriangleArea(t *testing.T) {
	tri := Triangle{A: 3, B: 4, C: 5}
	if !floatEqual(tri.Area(), 6) {
		t.Errorf("Tri(3,4,5).Area() = %f，期望 6", tri.Area())
	}
}

func TestTrianglePerimeter(t *testing.T) {
	tri := Triangle{A: 3, B: 4, C: 5}
	if !floatEqual(tri.Perimeter(), 12) {
		t.Errorf("Tri(3,4,5).Perimeter() = %f，期望 12", tri.Perimeter())
	}
}

func TestDescribe(t *testing.T) {
	tests := []struct {
		shape    Shape
		expected string
	}{
		{Circle{Radius: 5}, "Circle with radius 5"},
		{Rectangle{Width: 3, Height: 4}, "Rectangle 3x4"},
		{Triangle{A: 3, B: 4, C: 5}, "Triangle with sides 3, 4, 5"},
	}
	for _, tt := range tests {
		result := Describe(tt.shape)
		if result != tt.expected {
			t.Errorf("Describe(%v) = %q，期望 %q", tt.shape, result, tt.expected)
		}
	}
}

func TestTotalArea(t *testing.T) {
	shapes := []Shape{
		Circle{Radius: 1},              // pi
		Rectangle{Width: 2, Height: 3}, // 6
	}
	expected := math.Pi + 6
	result := TotalArea(shapes)
	if !floatEqual(result, expected) {
		t.Errorf("TotalArea = %f，期望 %f", result, expected)
	}
}

func TestTotalArea_Empty(t *testing.T) {
	if !floatEqual(TotalArea(nil), 0) {
		t.Error("空切片总面积应为0")
	}
}

func TestFilterByType(t *testing.T) {
	shapes := []Shape{
		Circle{Radius: 1},
		Rectangle{Width: 2, Height: 3},
		Circle{Radius: 5},
		Triangle{A: 3, B: 4, C: 5},
	}

	circles := FilterByType(shapes, "circle")
	if len(circles) != 2 {
		t.Errorf("应有2个圆形，得到 %d", len(circles))
	}

	rects := FilterByType(shapes, "rectangle")
	if len(rects) != 1 {
		t.Errorf("应有1个矩形，得到 %d", len(rects))
	}

	tris := FilterByType(shapes, "triangle")
	if len(tris) != 1 {
		t.Errorf("应有1个三角形，得到 %d", len(tris))
	}

	unknown := FilterByType(shapes, "hexagon")
	if len(unknown) != 0 {
		t.Errorf("不存在的类型应返回空，得到 %d", len(unknown))
	}
}
