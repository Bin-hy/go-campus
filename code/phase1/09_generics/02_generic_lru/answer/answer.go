//go:build ignore

package answer

type node[K comparable, V any] struct {
	key        K
	value      V
	prev, next *node[K, V]
}

type GenericLRU[K comparable, V any] struct {
	capacity int
	data     map[K]*node[K, V]
	head     *node[K, V]
	tail     *node[K, V]
}

func NewGenericLRU[K comparable, V any](capacity int) *GenericLRU[K, V] {
	head := &node[K, V]{}
	tail := &node[K, V]{}
	head.next = tail
	tail.prev = head
	return &GenericLRU[K, V]{
		capacity: capacity,
		data:     make(map[K]*node[K, V]),
		head:     head,
		tail:     tail,
	}
}

func (c *GenericLRU[K, V]) Get(key K) (V, bool) {
	if n, ok := c.data[key]; ok {
		c.moveToFront(n)
		return n.value, true
	}
	var zero V
	return zero, false
}

func (c *GenericLRU[K, V]) Put(key K, value V) {
	if n, ok := c.data[key]; ok {
		n.value = value
		c.moveToFront(n)
		return
	}
	n := &node[K, V]{key: key, value: value}
	c.data[key] = n
	c.addToFront(n)
	if len(c.data) > c.capacity {
		removed := c.tail.prev
		c.removeNode(removed)
		delete(c.data, removed.key)
	}
}

func (c *GenericLRU[K, V]) Len() int { return len(c.data) }

func (c *GenericLRU[K, V]) Delete(key K) {
	if n, ok := c.data[key]; ok {
		c.removeNode(n)
		delete(c.data, key)
	}
}

func (c *GenericLRU[K, V]) addToFront(n *node[K, V]) {
	n.prev = c.head
	n.next = c.head.next
	c.head.next.prev = n
	c.head.next = n
}

func (c *GenericLRU[K, V]) removeNode(n *node[K, V]) {
	n.prev.next = n.next
	n.next.prev = n.prev
}

func (c *GenericLRU[K, V]) moveToFront(n *node[K, V]) {
	c.removeNode(n)
	c.addToFront(n)
}
