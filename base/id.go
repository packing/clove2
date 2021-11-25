package base

import (
	"encoding/binary"
	"os"
	"sync"
)

type CloveId struct {
	a uint16
	b uint16
	c uint16
	d uint16
}

func (id CloveId) Integer() uint64 {
	return uint64(id.d)<<48 | uint64(id.c)<<32 | uint64(id.a)<<16 | uint64(id.b)
}

func (id CloveId) Data() []byte {
	var b = make([]byte, 8)
	binary.BigEndian.PutUint64(b, id.Integer())
	return b
}

func (id CloveId) DataWithByteOrder(byteOrder binary.ByteOrder) []byte {
	var b = make([]byte, 8)
	byteOrder.PutUint64(b, id.Integer())
	return b
}

type IdGenerator struct {
	seed  uint
	mutex sync.Mutex
}

func (idGen *IdGenerator) NextId() <-chan CloveId {
	var iv = make(chan CloveId)
	go func() {
		idGen.mutex.Lock()
		defer idGen.mutex.Unlock()

		var id = CloveId{}
		var next = idGen.seed + 1
		var pid = os.Getpid()
		id.a = uint16(pid & 0xffff)
		id.b = uint16(pid >> 16)
		id.c = uint16(next & 0xffff)
		id.d = uint16(next >> 16)

		iv <- id
		close(iv)
	}()
	return iv
}

func (idGen *IdGenerator) NextIdWithSeed(seed uint) <-chan CloveId {
	var iv = make(chan CloveId)
	go func() {
		idGen.mutex.Lock()
		defer idGen.mutex.Unlock()

		var id = CloveId{}
		var next = seed
		var pid = os.Getpid()
		id.a = uint16(pid & 0xffff)
		id.b = uint16(pid >> 16)
		id.c = uint16(next & 0xffff)
		id.d = uint16(next >> 16)

		iv <- id
		close(iv)
	}()
	return iv
}

var instIdGenerator *IdGenerator
var onceForIdGeneratorSingleton sync.Once

func GetIdGenerator() *IdGenerator {
	onceForIdGeneratorSingleton.Do(func() {
		instIdGenerator = &IdGenerator{}
	})
	return instIdGenerator
}
