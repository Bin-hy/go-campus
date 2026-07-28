//go:build ignore

package answer

type node struct {
	key, val   int
	prev, next *node
}

type LRUCache struct {
	capacity   int
	size       int
	cache      map[int]*node
	head, tail *node
}

func NewLRUCache(capacity int) *LRUCache {
	head := &node{}
	tail := &node{}
	head.next = tail
	tail.prev = head
	return &LRUCache{
		capacity: capacity,
		cache:    make(map[int]*node),
		head:     head,
		tail:     tail,
	}
}

func (c *LRUCache) Get(key int) int {
	if n, ok := c.cache[key]; ok {
		c.moveToHead(n)
		return n.val
	}
	return -1
}

func (c *LRUCache) Put(key, value int) {
	if n, ok := c.cache[key]; ok {
		n.val = value
		c.moveToHead(n)
		return
	}
	n := &node{key: key, val: value}
	c.cache[key] = n
	c.addToHead(n)
	c.size++
	if c.size > c.capacity {
		removed := c.removeTail()
		delete(c.cache, removed.key)
		c.size--
	}
}

func (c *LRUCache) addToHead(n *node) {
	n.prev = c.head
	n.next = c.head.next
	c.head.next.prev = n
	c.head.next = n
}

func (c *LRUCache) removeNode(n *node) {
	n.prev.next = n.next
	n.next.prev = n.prev
}

func (c *LRUCache) moveToHead(n *node) {
	c.removeNode(n)
	c.addToHead(n)
}

func (c *LRUCache) removeTail() *node {
	n := c.tail.prev
	c.removeNode(n)
	return n
}
