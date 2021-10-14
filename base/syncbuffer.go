package base

import (
	"bytes"
	"sync"
)

type SyncBuffer struct {
	b  bytes.Buffer
	rw sync.Mutex
}

func (b *SyncBuffer) Read(p []byte) (int, error) {
	b.rw.Lock()
	defer b.rw.Unlock()
	return b.b.Read(p)
}

func (b *SyncBuffer) Next(n int) ([]byte, int) {
	b.rw.Lock()
	defer b.rw.Unlock()
	var sn = n
	var bl = b.b.Len()
	if bl < n {
		sn = bl
	}
	return b.b.Next(sn), sn
}

func (b *SyncBuffer) Peek(n int) ([]byte, int) {
	b.rw.Lock()
	defer b.rw.Unlock()
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
	b.rw.Lock()
	defer b.rw.Unlock()
	return b.b.Len()
}
