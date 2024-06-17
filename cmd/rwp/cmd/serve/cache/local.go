package cache

// Originally from https://github.com/go-redis/cache/blob/v8.4.3/local.go
// Modified to store interface{} instead of []byte

import (
	"sync"
	"time"

	"github.com/vmihailenco/go-tinylfu"
	"golang.org/x/exp/rand"
)

type Evictable interface {
	OnEvict()
}

type LocalCache interface {
	Set(key string, data Evictable)
	Get(key string) (Evictable, bool)
	Del(key string)
}

type TinyLFU struct {
	mu     sync.Mutex
	rand   *rand.Rand
	lfu    *tinylfu.T
	ttl    time.Duration
	offset time.Duration
}

var _ LocalCache = (*TinyLFU)(nil)

func NewTinyLFU(size int, ttl time.Duration) *TinyLFU {
	const maxOffset = 10 * time.Second

	offset := ttl / 10
	if offset > maxOffset {
		offset = maxOffset
	}

	return &TinyLFU{
		rand:   rand.New(rand.NewSource(uint64(time.Now().UnixNano()))),
		lfu:    tinylfu.New(size, 100000),
		ttl:    ttl,
		offset: offset,
	}
}

func (c *TinyLFU) UseRandomizedTTL(offset time.Duration) {
	c.offset = offset
}

func (c *TinyLFU) Set(key string, b Evictable) {
	c.mu.Lock()
	defer c.mu.Unlock()

	ttl := c.ttl
	if c.offset > 0 {
		ttl += time.Duration(c.rand.Int63n(int64(c.offset)))
	}

	c.lfu.Set(&tinylfu.Item{
		Key:      key,
		Value:    b,
		ExpireAt: time.Now().Add(ttl),
		OnEvict: func() {
			b.OnEvict()
		},
	})
}

func (c *TinyLFU) Get(key string) (Evictable, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	val, ok := c.lfu.Get(key)
	if !ok {
		return nil, false
	}

	return val.(Evictable), true
}

func (c *TinyLFU) Del(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lfu.Del(key)
}
