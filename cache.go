package gcache

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	// TypeSimple represents simple cache
	TypeSimple int = iota
	// TypeLRU represents LRU cache
	TypeLRU
	// TypeLFU represents LFU cache
	TypeLFU
	// TypeARC represents ARC cache
	TypeARC
)

// ErrKeyNotFound is returned when key is not found in cache
var ErrKeyNotFound = errors.New("key not found")

// Cache interface is implemeted by all individual caches
type Cache interface {
	Set(key, value interface{}) error
	SetWithTTL(key, value interface{}, expiration time.Duration) error
	Get(key interface{}) (interface{}, error)
	GetIFPresent(key interface{}) (interface{}, error)
	GetALL(checkExpired bool) map[interface{}]interface{}
	get(key interface{}, onLoad bool) (interface{}, error)
	Remove(key interface{}) bool
	Purge()
	Keys(checkExpired bool) []interface{}
	Len(checkExpired bool) int
	Has(key interface{}) bool

	statsAccessor
}

type baseCache struct {
	clock           Clock
	size            int
	mu              sync.RWMutex
	defaultTTL      *time.Duration
	onAdd           func(key, value interface{})
	onDel           func(key, value interface{})
	onEvict         func(key, value interface{})
	onMiss          func(key interface{})
	onPurge         func(key, value interface{})
	deserializeWith func(interface{}, interface{}) (interface{}, error)
	serializeWith   func(interface{}, interface{}) (interface{}, error)
	loadGroup       Group
	*stats
}

type Config struct {
	cacheType int
	// Size config option is used to provide size of cache
	Size            int
	DefaultTTL      time.Duration
	OnAdd           func(key, value interface{})
	OnDel           func(key, value interface{})
	OnEvict         func(key, value interface{})
	OnMiss          func(key interface{})
	OnPurge         func(key, value interface{})
	DeserializeWith func(interface{}, interface{}) (interface{}, error)
	SerializeWith   func(interface{}, interface{}) (interface{}, error)
}

func buildCache(c *baseCache, cb Config) {
	c.clock = NewRealClock()
	c.defaultTTL = &cb.DefaultTTL
	c.size = cb.Size
	c.onAdd = cb.OnAdd
	c.onDel = cb.OnDel
	c.onEvict = cb.OnEvict
	c.onMiss = cb.OnMiss
	c.onPurge = cb.OnPurge
	c.stats = &stats{}
}

// load a new value using by specified key.
func (c *baseCache) load(key interface{}, cb func(interface{}, *time.Duration, error) (interface{}, error), isWait bool) (interface{}, bool, error) {
	v, called, err := c.loadGroup.Do(key, func() (v interface{}, e error) {
		defer func() {
			if r := recover(); r != nil {
				e = fmt.Errorf("Loader panics: %v", r)
			}
		}()
		return cb(c.loaderExpireFunc(key))
	}, isWait)
	if err != nil {
		return nil, called, err
	}
	return v, called, nil
}
