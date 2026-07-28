//go:build ignore

package answer

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"sync"
)

type BufferPool struct {
	pool sync.Pool
}

func NewBufferPool() *BufferPool {
	return &BufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				return new(bytes.Buffer)
			},
		},
	}
}

func (p *BufferPool) GetBuffer() *bytes.Buffer {
	return p.pool.Get().(*bytes.Buffer)
}

func (p *BufferPool) PutBuffer(buf *bytes.Buffer) {
	buf.Reset()
	p.pool.Put(buf)
}

func ProcessRequests(requests [][]byte) []string {
	if len(requests) == 0 {
		return []string{}
	}

	pool := NewBufferPool()
	results := make([]string, len(requests))

	for i, req := range requests {
		buf := pool.GetBuffer()
		fmt.Fprintf(buf, "processed: %s", hex.EncodeToString(req))
		results[i] = buf.String()
		pool.PutBuffer(buf)
	}

	return results
}
