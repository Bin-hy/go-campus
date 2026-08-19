//go:build ignore

package answer

// Loop 参考答案：事件循环（LT / ET 触发语义）
type Loop struct {
	handlers map[int]func(fd int)
	ready    map[int]bool
	et       map[int]bool
}

func NewLoop() *Loop {
	return &Loop{
		handlers: make(map[int]func(fd int)),
		ready:    make(map[int]bool),
		et:       make(map[int]bool),
	}
}

func (l *Loop) Add(fd int, handler func(fd int), edgeTriggered bool) {
	l.handlers[fd] = handler
	l.et[fd] = edgeTriggered
}

func (l *Loop) Remove(fd int) {
	delete(l.handlers, fd)
	delete(l.ready, fd)
	delete(l.et, fd)
}

func (l *Loop) Notify(fd int) {
	if _, ok := l.handlers[fd]; ok {
		l.ready[fd] = true
	}
}

func (l *Loop) Consume(fd int) {
	delete(l.ready, fd)
}

func (l *Loop) Process() {
	for fd := range l.ready {
		h, ok := l.handlers[fd]
		if !ok {
			continue
		}
		h(fd)
		if l.et[fd] {
			delete(l.ready, fd)
		}
	}
}
