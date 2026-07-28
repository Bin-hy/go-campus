//go:build ignore

package answer

type node struct {
	key, value int
	prev, next *node
}

type LRUCache struct {
	capacity int
	data     map[int]*node
	head     *node // dummy head (最新)
	tail     *node // dummy tail (最旧)
}

func NewLRUCache(capacity int) *LRUCache {
	head := &node{}
	tail := &node{}
	head.next = tail
	tail.prev = head
	return &LRUCache{
		capacity: capacity,
		data:     make(map[int]*node),
		head:     head,
		tail:     tail,
	}
}

func (c *LRUCache) Get(key int) int {
	if n, ok := c.data[key]; ok {
		c.moveToFront(n)
		return n.value
	}
	return -1
}

func (c *LRUCache) Put(key int, value int) {
	if n, ok := c.data[key]; ok {
		n.value = value
		c.moveToFront(n)
		return
	}

	n := &node{key: key, value: value}
	c.data[key] = n
	c.addToFront(n)

	if len(c.data) > c.capacity {
		removed := c.tail.prev
		c.removeNode(removed)
		delete(c.data, removed.key)
	}
}

func (c *LRUCache) addToFront(n *node) {
	n.prev = c.head
	n.next = c.head.next
	c.head.next.prev = n
	c.head.next = n
}

func (c *LRUCache) removeNode(n *node) {
	n.prev.next = n.next
	n.next.prev = n.prev
}

func (c *LRUCache) moveToFront(n *node) {
	c.removeNode(n)
	c.addToFront(n)
}
