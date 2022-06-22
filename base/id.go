package base

import (
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"io"
	"net"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/shirou/gopsutil/cpu"
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

type MachineID struct {
	cpuId  []string
	hwAddr []string
}

func GetMachineID() *MachineID {
	id := new(MachineID)
	infos, err := cpu.Info()
	if err == nil {
		id.cpuId = make([]string, 0)
		for _, info := range infos {
			if len(info.PhysicalID) > 0 {
				id.cpuId = append(id.cpuId, info.PhysicalID)
			}
		}
	} else {
		id.cpuId = make([]string, 0)
	}

	nets, err := net.Interfaces()
	if err == nil {
		id.hwAddr = make([]string, 0)
		for _, netinfo := range nets {
			if len(netinfo.HardwareAddr.String()) > 0 {
				id.hwAddr = append(id.hwAddr, netinfo.HardwareAddr.String())
			}
		}
	} else {
		id.hwAddr = make([]string, 0)
	}

	return id
}

func (id MachineID) GetHash() []byte {
	if len(id.hwAddr) == 0 {
		return []byte("")
	}
	s := new(strings.Builder)
	sort.Slice(id.cpuId, func(i, j int) bool {
		return id.cpuId[i] < id.cpuId[j]
	})
	sort.Slice(id.hwAddr, func(i, j int) bool {
		return id.hwAddr[i] < id.hwAddr[j]
	})
	for _, cpuId := range id.cpuId {
		s.WriteString(cpuId)
		s.WriteString("|")
	}
	for _, hwAddr := range id.hwAddr {
		s.WriteString(hwAddr)
		s.WriteString("+")
	}
	h := sha1.New()
	_, _ = io.WriteString(h, s.String())
	return h.Sum(nil)
}

func (id MachineID) GetHashString() string {
	return hex.EncodeToString(id.GetHash())
}

func (id MachineID) GetCPUID() []string {
	return id.cpuId
}

func (id MachineID) GetHWADDR() []string {
	return id.hwAddr
}
