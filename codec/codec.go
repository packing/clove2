/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package codec

import (
	"encoding/binary"
	"math"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/packing/clove2/errors"
)

const (
	ProtocolMemory   = 0x0
	ProtocolClove    = 0x1
	ProtocolJSON     = 0x2
	ProtocolReserved = 0xF
)

type CloveData = interface{}
type CloveMap = map[CloveData]CloveData
type CloveStrMap = map[string]CloveData
type CloveSlice = []CloveData

type Decoder interface {
	SetByteOrder(binary.ByteOrder)
	Decode([]byte) (error, CloveData, []byte)
}

type Encoder interface {
	SetByteOrder(binary.ByteOrder)
	Encode(*CloveData) (error, []byte)
}

type Codec struct {
	Protocol byte
	Version  byte
	Decoder  Decoder
	Encoder  Encoder
	Name     string
}

type DecoderMemory struct{}

func (receiver *DecoderMemory) SetByteOrder(binary.ByteOrder) {}
func (receiver DecoderMemory) Decode(raw []byte) (error, CloveData, []byte) {
	return nil, raw, raw[len(raw):]
}

type EncoderMemory struct{}

func (receiver *EncoderMemory) SetByteOrder(binary.ByteOrder) {}
func (receiver EncoderMemory) Encode(raw *CloveData) (error, []byte) {
	data, ok := (*raw).([]byte)
	if !ok {
		return errors.Errorf("Type is not supported"), nil
	}
	return nil, data
}

type CloveMapReader struct {
	Map CloveMap
}

func CreateMapReader(m CloveMap) *CloveMapReader {
	mr := new(CloveMapReader)
	mr.Map = m
	return mr
}

func (receiver CloveMapReader) TryReadValue(key interface{}) interface{} {
	kind := reflect.TypeOf(key).Kind()
	switch kind {
	case reflect.String:
		return receiver.Map[key]

	case reflect.Int:
		fallthrough
	case reflect.Int8:
		fallthrough
	case reflect.Int16:
		fallthrough
	case reflect.Int32:
		fallthrough
	case reflect.Int64:
		intV := reflect.ValueOf(key).Int()
		v, ok := receiver.Map[int(intV)]
		if ok {
			return v
		}

		v, ok = receiver.Map[int8(intV)]
		if ok {
			return v
		}

		v, ok = receiver.Map[int16(intV)]
		if ok {
			return v
		}

		v, ok = receiver.Map[int32(intV)]
		if ok {
			return v
		}

		v, ok = receiver.Map[int64(intV)]
		if ok {
			return v
		}

		v, ok = receiver.Map[uint(intV)]
		if ok {
			return v
		}

		v, ok = receiver.Map[uint8(intV)]
		if ok {
			return v
		}

		v, ok = receiver.Map[uint16(intV)]
		if ok {
			return v
		}

		v, ok = receiver.Map[uint32(intV)]
		if ok {
			return v
		}

		v, ok = receiver.Map[uint64(intV)]
		if ok {
			return v
		} else {
			return nil
		}

	case reflect.Uint:
		fallthrough
	case reflect.Uint8:
		fallthrough
	case reflect.Uint16:
		fallthrough
	case reflect.Uint32:
		fallthrough
	case reflect.Uint64:
		intV := reflect.ValueOf(key).Uint()
		v, ok := receiver.Map[uint(intV)]
		if ok {
			return v
		}

		v, ok = receiver.Map[uint8(intV)]
		if ok {
			return v
		}

		v, ok = receiver.Map[uint16(intV)]
		if ok {
			return v
		}

		v, ok = receiver.Map[uint32(intV)]
		if ok {
			return v
		}

		v, ok = receiver.Map[uint64(intV)]
		if ok {
			return v
		}

		v, ok = receiver.Map[int(intV)]
		if ok {
			return v
		}
		v, ok = receiver.Map[int8(intV)]
		if ok {
			return v
		}

		v, ok = receiver.Map[int16(intV)]
		if ok {
			return v
		}

		v, ok = receiver.Map[int32(intV)]
		if ok {
			return v
		}

		v, ok = receiver.Map[int64(intV)]
		if ok {
			return v
		} else {
			return nil
		}

	case reflect.Float32:
		fallthrough
	case reflect.Float64:
		floatV := reflect.ValueOf(key).Float()

		v, ok := receiver.Map[floatV]
		if ok {
			return v
		}

		v, ok = receiver.Map[float32(floatV)]
		if ok {
			return v
		} else {
			return nil
		}

	default:
		return nil
	}
}

func (receiver CloveMapReader) IntValueOf(key interface{}, def int64) int64 {
	v := receiver.TryReadValue(key)
	if v == nil {
		return def
	}
	switch v.(type) {
	case int:
		return int64(v.(int))
	case int8:
		return int64(v.(int8))
	case int16:
		return int64(v.(int16))
	case int32:
		return int64(v.(int32))
	case uint64:
		return int64(v.(uint64))
	case uint:
		return int64(v.(uint))
	case uint8:
		return int64(v.(uint8))
	case uint16:
		return int64(v.(uint16))
	case uint32:
		return int64(v.(uint32))
	case int64:
		return v.(int64)
	default:
		return def
	}
}

func (receiver CloveMapReader) IntegerNumberOf(key interface{}, def int64) int64 {
	v := receiver.TryReadValue(key)
	if v == nil {
		return def
	}
	switch v.(type) {
	case int:
		return int64(v.(int))
	case int8:
		return int64(v.(int8))
	case int16:
		return int64(v.(int16))
	case int32:
		return int64(v.(int32))
	case uint64:
		return int64(v.(uint64))
	case uint:
		return int64(v.(uint))
	case uint8:
		return int64(v.(uint8))
	case uint16:
		return int64(v.(uint16))
	case uint32:
		return int64(v.(uint32))
	case int64:
		return v.(int64)
	case float64:
		return int64(math.Floor(v.(float64)))
	case float32:
		return int64(math.Floor(float64(v.(float32))))
	case string:
		ir, err := strconv.ParseInt(v.(string), 10, 64)
		if err == nil {
			return ir
		} else {
			return def
		}
	default:
		return def
	}
}

func (receiver CloveMapReader) UintValueOf(key interface{}, def uint64) uint64 {
	v := receiver.TryReadValue(key)
	if v == nil {
		return def
	}
	switch v.(type) {
	case int:
		return uint64(v.(int))
	case int8:
		return uint64(v.(int8))
	case int16:
		return uint64(v.(int16))
	case int32:
		return uint64(v.(int32))
	case int64:
		return uint64(v.(int64))
	case uint:
		return uint64(v.(uint))
	case uint8:
		return uint64(v.(uint8))
	case uint16:
		return uint64(v.(uint16))
	case uint32:
		return uint64(v.(uint32))
	case uint64:
		return v.(uint64)
	default:
		return def
	}
}

func (receiver CloveMapReader) StrValueOf(key interface{}, def string) string {
	v := receiver.TryReadValue(key)
	if v == nil {
		return def
	}
	if reflect.TypeOf(v).Kind() != reflect.String {
		return def
	}
	return reflect.ValueOf(v).String()
}

func (receiver CloveMapReader) FloatValueOf(key interface{}, def float64) float64 {
	v := receiver.TryReadValue(key)
	if v == nil {
		return def
	}
	if reflect.TypeOf(v).Kind() != reflect.Float64 && reflect.TypeOf(v).Kind() != reflect.Float32 {
		return def
	}
	return reflect.ValueOf(v).Float()
}

func (receiver CloveMapReader) BoolValueOf(key interface{}) bool {
	v := receiver.TryReadValue(key)
	if v == nil {
		return false
	}
	if reflect.TypeOf(v).Kind() != reflect.Bool {
		return false
	}
	return reflect.ValueOf(v).Bool()
}

type CloveSliceReader struct {
	List CloveSlice
}

func CreateSliceReader(m CloveSlice) *CloveSliceReader {
	mr := new(CloveSliceReader)
	mr.List = m
	return mr
}

func (receiver CloveSliceReader) IntValueOf(index int, def int64) int64 {
	if index >= len(receiver.List) || index < 0 {
		return def
	}
	v := receiver.List[index]
	if v == nil {
		return def
	}
	switch v.(type) {
	case int:
		return int64(v.(int))
	case int8:
		return int64(v.(int8))
	case int16:
		return int64(v.(int16))
	case int32:
		return int64(v.(int32))
	case uint64:
		return int64(v.(uint64))
	case uint:
		return int64(v.(uint))
	case uint8:
		return int64(v.(uint8))
	case uint16:
		return int64(v.(uint16))
	case uint32:
		return int64(v.(uint32))
	case int64:
		return v.(int64)
	default:
		return def
	}
}

func (receiver CloveSliceReader) IntegerNumberOf(index int, def int64) int64 {
	if index >= len(receiver.List) || index < 0 {
		return def
	}
	v := receiver.List[index]
	if v == nil {
		return def
	}
	switch v.(type) {
	case int:
		return int64(v.(int))
	case int8:
		return int64(v.(int8))
	case int16:
		return int64(v.(int16))
	case int32:
		return int64(v.(int32))
	case uint64:
		return int64(v.(uint64))
	case uint:
		return int64(v.(uint))
	case uint8:
		return int64(v.(uint8))
	case uint16:
		return int64(v.(uint16))
	case uint32:
		return int64(v.(uint32))
	case int64:
		return v.(int64)
	case float64:
		return int64(math.Floor(v.(float64)))
	case float32:
		return int64(math.Floor(float64(v.(float32))))
	case string:
		ir, err := strconv.ParseInt(v.(string), 10, 64)
		if err == nil {
			return ir
		} else {
			return def
		}
	default:
		return def
	}
}

func (receiver CloveSliceReader) UintValueOf(index int, def uint64) uint64 {
	if index >= len(receiver.List) || index < 0 {
		return def
	}
	v := receiver.List[index]
	if v == nil {
		return def
	}
	switch v.(type) {
	case int:
		return uint64(v.(int))
	case int8:
		return uint64(v.(int8))
	case int16:
		return uint64(v.(int16))
	case int32:
		return uint64(v.(int32))
	case int64:
		return uint64(v.(int64))
	case uint:
		return uint64(v.(uint))
	case uint8:
		return uint64(v.(uint8))
	case uint16:
		return uint64(v.(uint16))
	case uint32:
		return uint64(v.(uint32))
	case uint64:
		return v.(uint64)
	default:
		return def
	}
}

func (receiver CloveSliceReader) StrValueOf(index int, def string) string {
	if index >= len(receiver.List) || index < 0 {
		return def
	}
	v := receiver.List[index]
	if reflect.TypeOf(v).Kind() != reflect.String {
		return def
	}
	return reflect.ValueOf(v).String()
}

func (receiver CloveSliceReader) MySQLStrValueOf(index int, def string) string {
	if index >= len(receiver.List) || index < 0 {
		return def
	}
	v := receiver.List[index]
	if v == nil {
		return def
	}
	if reflect.TypeOf(v).Kind() != reflect.Slice {
		return def
	}
	return string(reflect.ValueOf(v).Bytes())
}

func (receiver CloveSliceReader) FloatValueOf(index int, def float64) float64 {
	if index >= len(receiver.List) || index < 0 {
		return def
	}
	v := receiver.List[index]
	if reflect.TypeOf(v).Kind() != reflect.Float64 && reflect.TypeOf(v).Kind() != reflect.Float32 {
		return def
	}
	return reflect.ValueOf(v).Float()
}

func (receiver CloveSliceReader) BoolValueOf(index int) bool {
	if index >= len(receiver.List) || index < 0 {
		return false
	}
	v := receiver.List[index]
	if reflect.TypeOf(v).Kind() != reflect.Bool {
		return false
	}
	return reflect.ValueOf(v).Bool()
}

var MemoryCodec = Codec{Protocol: ProtocolMemory, Version: 1, Decoder: new(DecoderMemory), Encoder: new(EncoderMemory), Name: "memory"}

type Manager struct {
	mapCodecs map[string]*Codec
}

var instCodecManager *Manager
var onceForCodecManagerSingleton sync.Once

func GetCodecManager() *Manager {
	onceForCodecManagerSingleton.Do(func() {
		instCodecManager = &Manager{}
		instCodecManager.mapCodecs = make(map[string]*Codec)
		instCodecManager.AddCodec(&MemoryCodec)
		instCodecManager.AddCodec(&CloveCodec)
		instCodecManager.AddCodec(&JsonCodec)
	})
	return instCodecManager
}

func (f *Manager) AddCodec(codec *Codec) {
	f.mapCodecs[strings.ToLower(codec.Name)] = codec
}

func (f Manager) FindCodec(name string) *Codec {
	codec, ok := f.mapCodecs[strings.ToLower(name)]
	if ok {
		return codec
	}
	return nil
}
