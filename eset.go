package eset

import (
	"errors"
	"sync"
	"time"
	"unsafe"
)

const FACTOR = 6.5

type ExpirableSet struct {
	elems    map[interface{}]*base
	capacity int
	mutex    sync.RWMutex
}

type base struct {
	expireTime time.Time
}

// the underlying struct of map
type hmap struct {
	count      int   // live cells == size of ma
	flags      uint8
	B          uint8 // log_2 of buckets (can hold up to loadFactor * 2^B items)
}


func New() *ExpirableSet {
	es := &ExpirableSet{}
	es.init()
	return es
}


// Assigns a initial capacity to the set
// to reduce the performance consumption caused by expansion.
// Because the implementation of eset is map,
// its capacity expansion mechanism is the same as map,
// that is, when (capacity / 2^hmap.B) > loadFactor,
// the expansion will be triggered.
func NewWithCapacity(capacity int) *ExpirableSet{
	es := &ExpirableSet{}
	if capacity <= 8 {
		es.capacity = 8
	} else {
		// 13 is FACTOR * 2
		es.capacity = FACTOR * 2 << (capacity / 13)
	}

	es.init()
	return es
}


func(es *ExpirableSet) init() {
	if es.capacity > 0 {
		es.elems = make(map[interface{}]*base, es.capacity)
	} else {
		es.elems = make(map[interface{}]*base)
	}
}


func(es *ExpirableSet) buildBase(ttl time.Duration) *base {
	return &base{
		expireTime: time.Now().Add(ttl),
	}
}


func(es *ExpirableSet) add(elem interface{}, base *base) {
	es.elems[elem] = base
}


func(es *ExpirableSet) contains(elem interface{}) bool {
	_, isExist := es.elems[elem]
	return isExist
}


func(es *ExpirableSet) delExpiredElems() {
	for elem, base := range es.elems {
		if base.isExpired() {
			delete(es.elems, elem)
		}
	}
}


func(es *ExpirableSet) largerThan(other *ExpirableSet) bool {
	return len(es.elems) > len(other.elems)
}


// Add an element to the set normally.
// If the element is existed,
// its expiration time will be cleared if it has.
func(es *ExpirableSet) Add(elem interface{}) {
	es.mutex.Lock()
	es.add(elem, nil)
	es.mutex.Unlock()
}


// Add an element to the set with an expiration time.
// If the element is existed,
// its expiration time will be reset to new.
func(es *ExpirableSet) AddWithExpire(elem interface{}, expireTime time.Duration) {
	es.mutex.Lock()
	es.add(elem, es.buildBase(expireTime))
	es.mutex.Unlock()
}


// Update an existed element in the set,
// and its expiration time will be inherited.
// Returns an error if the element doesn't exist.
func(es *ExpirableSet) Update(old interface{}, new interface{}) (err error) {
	oldElem, isExist := es.elems[old]
	if isExist {
		es.mutex.Lock()
		es.elems[new] = oldElem
		delete(es.elems, old)
		es.mutex.Unlock()
	} else {
		err = errors.New("elem doesn't exist")
	}

	return
}


// Remove an element in the set.
// If the element doesn't exist, nothing will happen.
func(es *ExpirableSet) Remove(elem interface{}) {
	es.mutex.Lock()
	delete(es.elems, elem)
	es.mutex.Unlock()
}


// This method can release the deleted elements in memory.
// Although the manually removed and
// expired elements disappear in the set,
// they may not be released in memory for some reason.
func(es *ExpirableSet) ClearEvictedElems() {
	newElems := make(map[interface{}]*base)
	es.mutex.Lock()
	for elem, base := range es.elems {
		newElems[elem] = base
	}

	es.elems = newElems
	es.mutex.Unlock()
}


// Returns size and capacity of the set.
func(es *ExpirableSet) Info() (size, capacity int) {
	hmap := *(**hmap)(unsafe.Pointer(&es.elems))
	if hmap.B == 0 {
		return hmap.count, 8
	}

	return hmap.count, FACTOR * 2 << (int(hmap.B)-1)
}



// Get ttl of the element.
// Returns an error if the element doesn't exist,
// or if the element doesn't have ttl.
func(es *ExpirableSet) GetElemTTL(elem interface{}) (ttl float64, err error) {
	es.mutex.RLock()
	base, isExist := es.elems[elem]
	es.mutex.RUnlock()

	now := time.Now()
	ttl = -1
	if !isExist {
		err = errors.New("elem doesn't exist")
	} else if base == nil {
		err = errors.New("elem doesn't have ttl")
	} else if base.expireTime.After(now) {
		ttl = base.expireTime.Sub(now).Seconds()
	} else {
		err = errors.New("elem doesn't exist")
	}

	return ttl, err
}


// Returns a slice that has all unexpired elements.
func(es *ExpirableSet) GetAll() []interface{} {
	es.mutex.Lock()
	var tempSlice []interface{}
	for elem, base := range es.elems {
		if base.isExpired() {
			delete(es.elems, elem)
		} else {
			tempSlice = append(tempSlice, elem)
		}
	}

	es.mutex.Unlock()
	return tempSlice
}


func(es *ExpirableSet) Contains(elem interface{}) bool {
	es.mutex.RLock()
	base, isExist := es.elems[elem]
	es.mutex.RUnlock()
	return isExist && !base.isExpired()
}


func(es *ExpirableSet) Clear() {
	es.init()
}


// Returns true if the set is
// the subset of the other set.
func(es *ExpirableSet) IsSubSet(other *ExpirableSet) bool {
	if es.largerThan(other) {
		return false
	}

	es.mutex.RLock()
	other.mutex.RLock()
	for elem := range es.elems {
		if !other.contains(elem) {
			es.mutex.RUnlock()
			other.mutex.RUnlock()
			return false
		}
	}

	es.mutex.RUnlock()
	other.mutex.RUnlock()
	return true
}


func(es *ExpirableSet) Union(other *ExpirableSet) *ExpirableSet {
	lagerEs, smallEs := compareAndGet(es, other)
	smallEs.mutex.RLock()
	for elem := range smallEs.elems {
		if !lagerEs.contains(elem) {
			lagerEs.elems[elem] = smallEs.elems[elem]
		}
	}

	smallEs.mutex.RUnlock()
	return lagerEs
}


func(es *ExpirableSet) Intersect(other *ExpirableSet) *ExpirableSet {
	newEs := New()
	var lagerEs, smallEs *ExpirableSet
	if es.largerThan(other) {
		lagerEs, smallEs = es, other
	} else {
		lagerEs, smallEs = other, es
	}

	lagerEs.mutex.RLock()
	smallEs.mutex.RLock()
	for elem := range smallEs.elems {
		if lagerEs.contains(elem) {
			newEs.elems[elem] = smallEs.elems[elem]
		}
	}

	lagerEs.mutex.RUnlock()
	smallEs.mutex.RUnlock()
	return newEs
}


func(es *ExpirableSet) Different(other *ExpirableSet) *ExpirableSet {
	lagerEs, smallEs := compareAndGet(es, other)

	smallEs.mutex.RLock()
	for elem := range smallEs.elems {
		if lagerEs.contains(elem) {
			delete(lagerEs.elems, elem)
		} else {
			lagerEs.elems[elem] = smallEs.elems[elem]
		}
	}

	smallEs.mutex.RUnlock()
	return lagerEs
}


// Ignore the order to determine
// whether the elements in the set are equal.
func(es *ExpirableSet) Equal(other *ExpirableSet) bool {
	if len(es.elems) != len(other.elems) {
		return false
	}

	es.mutex.RLock()
	other.mutex.RLock()

	for elem := range other.elems {
		if !es.contains(elem) {
			es.mutex.RUnlock()
			other.mutex.RUnlock()
			return false
		}
	}

	es.mutex.RUnlock()
	other.mutex.RUnlock()
	return true
}


func(es *ExpirableSet) Clone() *ExpirableSet {
	return &ExpirableSet{
		elems:    es.elems,
		capacity: es.capacity,
	}
}


func(es *ExpirableSet) Size() int {
	es.mutex.Lock()
	es.delExpiredElems()
	es.mutex.Unlock()
	return len(es.elems)
}


// Do something for each elements in the set.
func(es *ExpirableSet) ForEach(handler func(interface{})) {
	es.mutex.Lock()
	for elem, base := range es.elems {
		if base.isExpired() {
			delete(es.elems, elem)
			continue
		}

		handler(elem)
	}
	es.mutex.Unlock()
}


func(b *base) isExpired() bool {
	return b != nil && b.expireTime.Before(time.Now())
}


// Compare two set's size.
// Returns the bigger one's clone and the smaller one.
func compareAndGet(one, other *ExpirableSet) (*ExpirableSet, *ExpirableSet) {
	if one.largerThan(other) {
		return one.Clone(), other
	}
	return other.Clone(), one
}
