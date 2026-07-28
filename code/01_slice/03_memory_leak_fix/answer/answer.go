//go:build ignore

package answer

func GetFirstN(data []byte, n int) []byte {
	if data == nil {
		return nil
	}
	if n <= 0 {
		return []byte{}
	}
	if n > len(data) {
		n = len(data)
	}
	result := make([]byte, n)
	copy(result, data[:n])
	return result
}

func FilterLargeSlice(data []int, predicate func(int) bool) []int {
	var result []int
	for _, v := range data {
		if predicate(v) {
			result = append(result, v)
		}
	}
	return result
}

func TrimMessage(msg []byte) []byte {
	if len(msg) < 6 {
		return nil
	}
	payload := msg[4 : len(msg)-2]
	result := make([]byte, len(payload))
	copy(result, payload)
	return result
}
