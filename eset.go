package eset

import (
	"sync"
	"time"
)

type ExpirableSet struct {
	elems    map[interface{}]*base
	capacity int
	mutex    sync.RWMutex
}

type base struct {
	time time.Time
}


func New() *ExpirableSet {
	es := &ExpirableSet{}
	es.init()
	return es
}


func NewWithCapacity(cap int) *ExpirableSet {
	es := &ExpirableSet{}
	es.capacity = cap
	es.init()
	return es
}


func(es *ExpirableSet) init() {
	if es.capacity != 0 {
		es.elems = make(map[interface{}]*base, es.capacity)
	} else {
		es.elems = make(map[interface{}]*base)
	}
}


func(es *ExpirableSet) buildBase(expireTime time.Duration) *base {
	return &base{
		time: time.Now().Add(expireTime),
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


func(es *ExpirableSet) Add(elem interface{}) {
	es.mutex.Lock()
	es.add(elem, nil)
	es.mutex.Unlock()
}


func(es *ExpirableSet) AddWithExpire(elem interface{}, expireTime time.Duration) {
	es.mutex.Lock()
	es.add(elem, es.buildBase(expireTime))
	es.mutex.Unlock()
}


func(es *ExpirableSet) Update(oldElem interface{}, newElem interface{}) {
	es.mutex.Lock()
	es.elems[newElem] = es.elems[oldElem]
	delete(es.elems, oldElem)
	es.mutex.Unlock()
}


func(es *ExpirableSet) Remove(elem interface{}) {
	es.mutex.Lock()
	delete(es.elems, elem)
	es.mutex.Unlock()
}


func(es *ExpirableSet) GetCapacity() int {
	return es.capacity
}


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
	es.mutex.Lock()
	base, isExist := es.elems[elem]
	es.mutex.Unlock()
	return isExist && !base.isExpired()
}


func(es *ExpirableSet) Clear() {
	es.init()
}


func(es *ExpirableSet) Union(other *ExpirableSet) *ExpirableSet {
	lagerEs, smallEs := compareAndGet(es, other)
	smallEs.mutex.Lock()
	for elem := range smallEs.elems {
		if !lagerEs.contains(elem) {
			lagerEs.elems[elem] = lagerEs.elems[elem]
		}
	}

	smallEs.mutex.Unlock()
	return lagerEs
}


func(es *ExpirableSet) Intersect(other *ExpirableSet) *ExpirableSet {
	newEs := New()
	var lagerEs, smallEs *ExpirableSet
	if es.largerThan(other) {
		lagerEs = es
		smallEs = other
	} else {
		lagerEs = other.Clone()
		smallEs = es
	}

	smallEs.mutex.Lock()
	for elem := range smallEs.elems {
		if lagerEs.contains(elem) {
			newEs.elems[elem] = smallEs.elems[elem]
		}
	}

	smallEs.mutex.Unlock()
	return newEs
}


func(es *ExpirableSet) Different(other *ExpirableSet) *ExpirableSet {
	lagerEs, smallEs := compareAndGet(es, other)

	smallEs.mutex.Lock()
	for elem := range smallEs.elems {
		if lagerEs.contains(elem) {
			delete(lagerEs.elems, elem)
		} else {
			lagerEs.elems[elem] = smallEs.elems[elem]
		}
	}

	smallEs.mutex.Unlock()
	return lagerEs
}


func(es *ExpirableSet) Equal(other *ExpirableSet) bool {
	if len(es.elems) != len(other.elems) {
		return false
	}

	es.mutex.Lock()
	other.mutex.Lock()

	for elem := range other.elems {
		if !es.contains(elem) {
			es.mutex.Unlock()
			other.mutex.Unlock()
			return false
		}
	}

	es.mutex.Unlock()
	other.mutex.Unlock()
	return true
}


func(es *ExpirableSet) Clone() *ExpirableSet {
	return &ExpirableSet{
		elems: es.elems,
		capacity: es.capacity,
	}
}


func(es *ExpirableSet) Size() int {
	es.mutex.Lock()
	es.delExpiredElems()
	es.mutex.Unlock()
	return len(es.elems)
}


func(b *base) isExpired() bool {
	return b != nil && b.time.Before(time.Now())
}


func compareAndGet(one, other *ExpirableSet) (*ExpirableSet, *ExpirableSet) {
	var lagerEs, smallEs *ExpirableSet
	if one.largerThan(other) {
		lagerEs = one.Clone()
		smallEs = other
	} else {
		lagerEs = other.Clone()
		smallEs = one
	}

	return lagerEs, smallEs
}
