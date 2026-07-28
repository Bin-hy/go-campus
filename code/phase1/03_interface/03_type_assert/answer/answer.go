//go:build ignore

package answer

import (
	"fmt"
	"math"
)

type Shape interface {
	Area() float64
	Perimeter() float64
}

type Circle struct{ Radius float64 }
type Rectangle struct{ Width, Height float64 }
type Triangle struct{ A, B, C float64 }

func (c Circle) Area() float64      { return math.Pi * c.Radius * c.Radius }
func (c Circle) Perimeter() float64  { return 2 * math.Pi * c.Radius }
func (r Rectangle) Area() float64    { return r.Width * r.Height }
func (r Rectangle) Perimeter() float64 { return 2 * (r.Width + r.Height) }
func (t Triangle) Area() float64 {
	s := (t.A + t.B + t.C) / 2
	return math.Sqrt(s * (s - t.A) * (s - t.B) * (s - t.C))
}
func (t Triangle) Perimeter() float64 { return t.A + t.B + t.C }

func Describe(s Shape) string {
	switch v := s.(type) {
	case Circle:
		return fmt.Sprintf("Circle with radius %g", v.Radius)
	case Rectangle:
		return fmt.Sprintf("Rectangle %gx%g", v.Width, v.Height)
	case Triangle:
		return fmt.Sprintf("Triangle with sides %g, %g, %g", v.A, v.B, v.C)
	default:
		return "Unknown shape"
	}
}

func TotalArea(shapes []Shape) float64 {
	var total float64
	for _, s := range shapes {
		total += s.Area()
	}
	return total
}

func FilterByType(shapes []Shape, typeName string) []Shape {
	var result []Shape
	for _, s := range shapes {
		switch typeName {
		case "circle":
			if _, ok := s.(Circle); ok { result = append(result, s) }
		case "rectangle":
			if _, ok := s.(Rectangle); ok { result = append(result, s) }
		case "triangle":
			if _, ok := s.(Triangle); ok { result = append(result, s) }
		}
	}
	return result
}
