package lru

import "container/list"

type Cache struct {
	// 允许使用的最大内存
	maxBytes int64
	//当前已使用的内存
	nbytes int64
	ll     *list.List
	// 键是字符串，值是双向链表中对应节点的指针
	cache map[string]*list.Element
	// OnEvicted 是某条记录被移除时的回调函数，可以为 nil
	OnEvicted func(key string, value Value)
}

// 双向链表节点的数据类型，在链表中仍保存每个值对应的 key 的好处在于，淘汰队首节点时，需要用 key 从字典中删除对应的映射
type entry struct {
	key   string
	value Value
}

type Value interface {
	// Len 返回值所占用的内存大小
	Len() int
}

func New(maxBytes int64, onEvicted func(string, Value)) *Cache {
	return &Cache{
		maxBytes:  maxBytes,
		ll:        list.New(),
		cache:     map[string]*list.Element{},
		OnEvicted: onEvicted,
	}
}

// Get 键对应的链表节点存在，则将对应节点移动到队首，并返回查找到的值
func (c *Cache) Get(key string) (Value, bool) {
	if ele, ok := c.cache[key]; ok {
		c.ll.MoveToFront(ele)
		kv := ele.Value.(*entry)
		return kv.value, true
	}
	return nil, false
}

// RemoveOldest 删除队尾节点，清空map并释放内存
func (c *Cache) RemoveOldest() {
	ele := c.ll.Back()
	if ele != nil {
		c.ll.Remove(ele)
		kv := ele.Value.(*entry)
		delete(c.cache, kv.key)
		c.nbytes -= int64(len(kv.key)) + int64(kv.value.Len())
		if c.OnEvicted != nil {
			c.OnEvicted(kv.key, kv.value)
		}
	}
}

// Add 如果键存在，则更新对应节点的值，并将该节点移到队尾。
// 不存在则是新增场景，首先队尾添加新节点 &entry{key, value}, 并字典中添加 key 和节点的映射关系。
// 更新 c.nbytes，如果超过了设定的最大值 c.maxBytes，则移除最少访问的节点
func (c *Cache) Add(key string, value Value) {
	if ele, ok := c.cache[key]; ok {
		c.ll.MoveToFront(ele)
		kv := ele.Value.(*entry)
		c.nbytes += int64(value.Len()) - int64(kv.value.Len())
		kv.value = value
	} else {
		ele = c.ll.PushFront(&entry{key: key, value: value})
		c.nbytes += int64(value.Len()) + int64(len(key))
		c.cache[key] = ele
	}
	for c.maxBytes != 0 && c.maxBytes < c.nbytes {
		c.RemoveOldest()
	}
}

func (c *Cache) Len() int {
	return c.ll.Len()
}
