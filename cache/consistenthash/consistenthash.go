package consistenthash

import (
	"hash/crc32"
	"sort"
	"strconv"
)

// Hash map bytes to uint32
type Hash func(data []byte) uint32

// Map contains all hashed keys
type Map struct {
	hash Hash
	// 虚拟节点倍数
	replicas int
	// 哈希环
	keys []int
	// 虚拟节点与真实节点的映射表
	hashMap map[int]string
}

func New(replicas int, fn Hash) *Map {
	m := &Map{
		replicas: replicas,
		hashMap:  map[int]string{},
	}
	if fn == nil {
		fn = crc32.ChecksumIEEE
	}
	m.hash = fn
	return m
}

func (m *Map) Add(keys ...string) {
	for _, key := range keys {
		for i := 0; i < m.replicas; i++ {
			hash := int(m.hash([]byte(strconv.Itoa(i) + key)))
			m.keys = append(m.keys, hash)
			m.hashMap[hash] = key
		}
	}
	sort.Ints(m.keys)
}

func (m *Map) Remove(key string) {
	for i := 0; i < m.replicas; i++ {
		hash := int(m.hash([]byte(strconv.Itoa(i) + key)))
		index := sort.SearchInts(m.keys, hash)
		m.keys = append(m.keys[:index], m.keys[index+1:]...)
		delete(m.hashMap, hash)
	}
}

func (m *Map) Get(key string) string {
	if len(key) == 0 {
		return ""
	}
	hash := int(m.hash([]byte(key)))
	index := sort.Search(len(m.keys), func(i int) bool {
		return m.keys[i] >= hash
	})
	// 根据虚拟节点的hash找到真正的节点
	return m.hashMap[m.keys[index%len(m.keys)]]
}
