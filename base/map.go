package base

import "sync"

//Dictionary key-value pair object
type KeyValue struct {
	Key   interface{}
	Value interface{}
}

//Extended map interface
type CloveMap interface {
	Set(key, value interface{})
	Get(key interface{}) interface{}

	Pop(key interface{}) interface{}
	Clear()

	HasKey(key interface{}) bool

	Count() int

	Keys() []interface{}
	Values() []interface{}
	Items() []KeyValue

	IterItems() <-chan KeyValue
}

//Extended map object
type dict struct {
	m     map[interface{}]interface{}
	rw    sync.RWMutex
	block bool
}

//Create a CloveMap object
func NewCloveMap(block bool) CloveMap {
	var sm = &dict{}
	sm.block = block
	sm.m = make(map[interface{}]interface{})
	return sm
}

//Create a sync CloveMap object
func NewSyncCloveMap() CloveMap {
	return NewCloveMap(true)
}

//Create an async CloveMap object
func NewAsyncCloveMap() CloveMap {
	return NewCloveMap(false)
}

func (d *dict) lock() {
	if d.block {
		d.rw.Lock()
	}
}

func (d *dict) unlock() {
	if d.block {
		d.rw.Unlock()
	}
}

func (d *dict) rLock() {
	if d.block {
		d.rw.RLock()
	}
}

func (d *dict) rUnlock() {
	if d.block {
		d.rw.RUnlock()
	}
}

func (d *dict) Set(key, value interface{}) {
	d.lock()
	defer d.unlock()

	d.m[key] = value
}

func (d *dict) Pop(key interface{}) interface{} {
	d.lock()
	defer d.unlock()

	val, _ := d.m[key]
	delete(d.m, key)
	return val
}

func (d *dict) Clear() {
	d.lock()
	defer d.unlock()

	for k := range d.m {
		delete(d.m, k)
	}
}

func (d *dict) HasKey(key interface{}) bool {
	d.rLock()
	defer d.rUnlock()

	_, ok := d.m[key]
	return ok
}

func (d *dict) GetWithDefault(key interface{}, def interface{}) interface{} {
	d.rLock()
	defer d.rUnlock()

	v, ok := d.m[key]
	if !ok {
		return def
	}
	return v
}

func (d *dict) Get(key interface{}) interface{} {
	return d.GetWithDefault(key, nil)
}

func (d *dict) Count() int {
	d.rLock()
	defer d.rUnlock()

	return len(d.m)
}

func (d *dict) Keys() []interface{} {
	d.rLock()
	defer d.rUnlock()

	var keys = make([]interface{}, 0, 0)
	for k := range d.m {
		keys = append(keys, k)
	}
	return keys
}

func (d *dict) Values() []interface{} {
	d.rLock()
	defer d.rUnlock()

	var values = make([]interface{}, 0, 0)
	for _, v := range d.m {
		values = append(values, v)
	}
	return values
}

func (d *dict) IterItems() <-chan KeyValue {
	var iv = make(chan KeyValue)

	go func(sd *dict) {
		sd.rLock()
		defer sd.rUnlock()

		for k, v := range sd.m {
			iv <- KeyValue{k, v}
		}

		close(iv)
	}(d)

	return iv
}

func (d *dict) Items() []KeyValue {
	d.rLock()
	defer d.rUnlock()

	var kvs = make([]KeyValue, 0, 0)
	for k, v := range d.m {
		kvs = append(kvs, KeyValue{k, v})
	}
	return kvs
}
