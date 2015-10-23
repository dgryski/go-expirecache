package expirecache

import (
	"math/rand"
	"sync"
	"time"
)

type element struct {
	validUntil time.Time
	data       interface{}
	size       uint64
}

type Cache struct {
	sync.Mutex
	cache     map[string]element
	keys      []string
	totalSize uint64
	maxSize   uint64
}

func New(maxSize uint64) *Cache {
	return &Cache{
		cache:   make(map[string]element),
		maxSize: maxSize,
	}
}

func (ec *Cache) Size() uint64 {
	ec.Lock()
	s := ec.totalSize
	ec.Unlock()
	return s
}

func (ec *Cache) Items() int {
	ec.Lock()
	k := len(ec.keys)
	ec.Unlock()
	return k
}

func (ec *Cache) Get(k string) (interface{}, bool) {
	ec.Lock()
	v, ok := ec.cache[k]
	ec.Unlock()
	if !ok || v.validUntil.Before(timeNow()) {
		// Can't actually delete this element from the cache here since
		// we can't remove the key from ec.keys without a linear search.
		// It'll get removed during the next cleanup
		return nil, false
	}
	return v.data, ok
}

func (ec *Cache) Set(k string, v interface{}, size uint64, expire int32) {
	ec.Lock()
	oldv, ok := ec.cache[k]
	if !ok {
		ec.keys = append(ec.keys, k)
	} else {
		ec.totalSize -= oldv.size
	}

	ec.totalSize += size
	ec.cache[k] = element{validUntil: timeNow().Add(time.Duration(expire) * time.Second), data: v, size: size}

	for ec.maxSize > 0 && ec.totalSize > ec.maxSize {
		ec.RandomEvict()
	}

	ec.Unlock()
}

func (ec *Cache) RandomEvict() {
	slot := rand.Intn(len(ec.keys))
	k := ec.keys[slot]

	ec.keys[slot] = ec.keys[len(ec.keys)-1]
	ec.keys = ec.keys[:len(ec.keys)-1]

	v := ec.cache[k]
	ec.totalSize -= v.size

	delete(ec.cache, k)
}

func (ec *Cache) Cleaner(d time.Duration) {

	for {
		cleanerSleep(d)

		now := timeNow()
		ec.Lock()

		// We could potentially be holding this lock for a long time,
		// but since we keep the cache expiration times small, we
		// expect only a small number of elements here to loop over

		for i := 0; i < len(ec.keys); i++ {
			k := ec.keys[i]
			v := ec.cache[k]
			if v.validUntil.Before(now) {
				ec.totalSize -= v.size
				delete(ec.cache, k)

				ec.keys[i] = ec.keys[len(ec.keys)-1]
				ec.keys = ec.keys[:len(ec.keys)-1]
				i-- // so we reprocess this index
			}
		}

		ec.Unlock()
		cleanerDone()
	}
}

var (
	timeNow      = time.Now
	cleanerSleep = time.Sleep
	cleanerDone  = func() {}
)
