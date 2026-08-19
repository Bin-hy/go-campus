//go:build ignore

package answer

import (
	"bufio"
	"context"
	"io"
	"strings"
)

// ReadSSE 参考答案
func ReadSSE(ctx context.Context, r io.Reader, onData func(string) error) error {
	scanner := bufio.NewScanner(r)
	var buf []string

	flush := func() error {
		if len(buf) == 0 {
			return nil
		}
		line := strings.Join(buf, "\n")
		buf = buf[:0]
		return onData(line)
	}

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		line := scanner.Text()
		if line == "" {
			if err := flush(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			d := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if d == "[DONE]" {
				return flush()
			}
			buf = append(buf, d)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return flush()
}
