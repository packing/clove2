package base

import (
	"bytes"
	"sync"
)

type Buffer interface {
	Read([]byte) (int, error)
	Next(int) ([]byte, int)
	Peek(int) ([]byte, int)
	Write([]byte) (int, error)
	Reset()
	Len() int
}

type StandardBuffer struct {
	b bytes.Buffer
}

func (b *StandardBuffer) Read(p []byte) (int, error) {
	return b.b.Read(p)
}

func (b *StandardBuffer) Next(n int) ([]byte, int) {
	var sn = n
	var bl = b.b.Len()
	if bl < n {
		sn = bl
	}
	return b.b.Next(sn), sn
}

func (b *StandardBuffer) Peek(n int) ([]byte, int) {
	var sn = n
	var bl = b.b.Len()
	if bl < n {
		sn = bl
	}
	return b.b.Bytes()[:sn], sn
}

func (b *StandardBuffer) Write(p []byte) (int, error) {
	return b.b.Write(p)
}

func (b *StandardBuffer) Reset() {
	b.b.Reset()
}

func (b *StandardBuffer) Len() int {
	return b.b.Len()
}

type SyncBuffer struct {
	b  bytes.Buffer
	rw sync.RWMutex
}

func (b *SyncBuffer) Read(p []byte) (int, error) {
	b.rw.RLock()
	defer b.rw.RUnlock()
	return b.b.Read(p)
}

func (b *SyncBuffer) Next(n int) ([]byte, int) {
	b.rw.RLock()
	defer b.rw.RUnlock()
	var sn = n
	var bl = b.b.Len()
	if bl < n {
		sn = bl
	}
	return b.b.Next(sn), sn
}

func (b *SyncBuffer) Peek(n int) ([]byte, int) {
	b.rw.RLock()
	defer b.rw.RUnlock()
	var sn = n
	var bl = b.b.Len()
	if bl < n {
		sn = bl
	}
	return b.b.Bytes()[:sn], sn
}

func (b *SyncBuffer) Write(p []byte) (int, error) {
	b.rw.Lock()
	defer b.rw.Unlock()
	return b.b.Write(p)
}

func (b *SyncBuffer) Reset() {
	b.rw.Lock()
	defer b.rw.Unlock()
	b.b.Reset()
}

func (b *SyncBuffer) Len() int {
	b.rw.RLock()
	defer b.rw.RUnlock()
	return b.b.Len()
}
