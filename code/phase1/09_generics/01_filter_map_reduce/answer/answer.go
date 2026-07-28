//go:build ignore

package answer

func Filter[T any](slice []T, fn func(T) bool) []T {
	var result []T
	for _, v := range slice {
		if fn(v) {
			result = append(result, v)
		}
	}
	if result == nil {
		return []T{}
	}
	return result
}

func Map[T any, U any](slice []T, fn func(T) U) []U {
	result := make([]U, len(slice))
	for i, v := range slice {
		result[i] = fn(v)
	}
	return result
}

func Reduce[T any, U any](slice []T, init U, fn func(U, T) U) U {
	acc := init
	for _, v := range slice {
		acc = fn(acc, v)
	}
	return acc
}

func Contains[T comparable](slice []T, target T) bool {
	for _, v := range slice {
		if v == target {
			return true
		}
	}
	return false
}

func Unique[T comparable](slice []T) []T {
	seen := make(map[T]struct{})
	var result []T
	for _, v := range slice {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			result = append(result, v)
		}
	}
	return result
}

func GroupBy[T any, K comparable](slice []T, keyFn func(T) K) map[K][]T {
	result := make(map[K][]T)
	for _, v := range slice {
		key := keyFn(v)
		result[key] = append(result[key], v)
	}
	return result
}
