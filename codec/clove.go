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

// @Title           clove.go
// @Description     Realize serialization and deserialization of Clove data and Golang data.
// @Author          starlove

package codec

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"time"
	"unsafe"

	"github.com/packing/clove2/base"
	"github.com/packing/clove2/errors"
)

var (
	ErrDLShort          = errors.Errorf("Data length too short to decode")
	ErrDHLShort         = errors.Errorf("Header data length too short")
	ErrDLNotMatch       = errors.Errorf("The data length does not match")
	ErrTypeNotSupported = errors.Errorf("Type is not supported")
)

/*

 Clove Protocol-Data struct

*/

const (
	CloveDataTypeUnknown   = 0
	CloveDataTypeInt8      = 1
	CloveDataTypeUint8     = 2
	CloveDataTypeInt16     = 3
	CloveDataTypeUint16    = 4
	CloveDataTypeInt32     = 5
	CloveDataTypeUint32    = 6
	CloveDataTypeInt64     = 7
	CloveDataTypeUint64    = 8
	CloveDataTypeFloat32   = 9
	CloveDataTypeFloat64   = 10
	CloveDataTypeNil       = 11
	CloveDataTypeTrue      = 12
	CloveDataTypeFalse     = 13
	CloveDataTypeString    = 14
	CloveDataTypeBytes     = 15
	CloveDataTypeMap       = 16
	CloveDataTypeList      = 17
	CloveDataTypeBigNumber = 18
	CloveDataTypeReserved  = 19
)

// DecoderClove 	Clove data deserialization class
type DecoderClove struct {
	byteOrder binary.ByteOrder
}

// EncoderClove 	Clove data serialization class
type EncoderClove struct {
	byteOrder binary.ByteOrder
}

var TimeCollector = make(map[string]int64)

func CollectTime(tag string, st *time.Time) {
	t := time.Now().Sub(*st).Nanoseconds()
	f, ok := TimeCollector[tag]
	if ok {
		TimeCollector[tag] = f + t
	} else {
		TimeCollector[tag] = t
	}
	*st = time.Now()
}

var CloveCodec = Codec{Protocol: ProtocolClove, Version: 1, Decoder: new(DecoderClove), Encoder: new(EncoderClove), Name: "clove"}

// calculateCloveTypeSize Calculates the length of memory needed to store the specified type of Clove data.
func calculateCloveTypeSize(tp byte) uint32 {
	switch tp {
	case CloveDataTypeInt8:
		fallthrough
	case CloveDataTypeUint8:
		return 1
	case CloveDataTypeInt16:
		fallthrough
	case CloveDataTypeUint16:
		return 2
	case CloveDataTypeFloat32:
		fallthrough
	case CloveDataTypeInt32:
		fallthrough
	case CloveDataTypeUint32:
		return 4
	case CloveDataTypeFloat64:
		fallthrough
	case CloveDataTypeInt64:
		fallthrough
	case CloveDataTypeUint64:
		return 8
	default:
		return 0
	}
}

// @title           makeHeader
// @description     Calculates the length of memory needed to store the specified type of Clove data.
// @auth            starlove
// @param           tp        	byte         "Type of Clove data"
// @return			uint32 		"The length of memory"
func makeHeader(tp byte, lenSize byte) byte {
	var h byte = 0x80
	h |= tp & 0x1f
	switch lenSize {
	case 1:
		h |= 0x20
	case 2:
		h |= 0x40
	case 4:
		h |= 0x60
	}
	return h
}

func (receiver EncoderClove) makeHeaderAndLength(tp byte, lenData int) []byte {
	var b = make([]byte, 5)
	var lenSize byte = 4
	var lenb = lenData
	switch {
	case lenb <= 0xff:
		lenSize = 1
		b[1] = byte(lenb)
	case lenb <= 0xffff:
		lenSize = 2
		receiver.getByteOrder().PutUint16(b[1:], uint16(lenb))
	default:
		lenSize = 4
		receiver.getByteOrder().PutUint32(b[1:], uint32(lenb))
	}
	b[0] = makeHeader(tp, lenSize)
	b = b[:1+lenSize]
	return b
}

func (receiver *DecoderClove) SetByteOrder(byteOrder binary.ByteOrder) {
	receiver.byteOrder = byteOrder
}

func (receiver DecoderClove) getByteOrder() binary.ByteOrder {
	if receiver.byteOrder == nil {
		return binary.BigEndian
	}
	return receiver.byteOrder
}

func (receiver DecoderClove) Decode(raw []byte) (error, CloveData, []byte) {
	defer func() {
		base.LogPanic(recover())
	}()
	if len(raw) == 0 {
		return ErrDLShort, nil, raw
	}
	if len(raw) < 1 {
		return ErrDHLShort, nil, raw
	}

	fByte := raw[0]
	opCode := fByte >> 7
	if opCode == 0 {
		return nil, int(fByte), raw[1:]
	}

	elementType := fByte & 0x1f
	realData := raw
	lenSizeType := (fByte >> 5) & 0x3
	var elementCount uint32 = 0
	switch lenSizeType {
	case 1:
		if len(raw) < 2 {
			return ErrDHLShort, nil, raw
		}
		realData = raw[2:]
		elementCount = uint32(raw[1])
	case 2:
		if len(raw) < 3 {
			return ErrDHLShort, nil, raw
		}
		realData = raw[3:]
		elementCount = uint32(receiver.getByteOrder().Uint16(raw[1:3]))
	case 3:
		if len(raw) < 5 {
			return ErrDHLShort, nil, raw
		}
		realData = raw[5:]
		elementCount = receiver.getByteOrder().Uint32(raw[1:5])
	default:
		realData = raw[1:]
		elementCount = calculateCloveTypeSize(elementType)
	}

	if elementType != CloveDataTypeMap && elementType != CloveDataTypeList {
		if uint32(len(realData)) < elementCount {
			//如果数据长度不符合预期，可以断定非法数据
			return ErrDHLShort, nil, nil
		}
	}

	switch elementType {
	case CloveDataTypeReserved:
	case CloveDataTypeNil:
		return nil, nil, realData[elementCount:]

	case CloveDataTypeBigNumber:
		e := string(realData[:elementCount])
		var rr interface{} = 0
		er, errpar := strconv.ParseInt(e, 10, 64)
		if errpar != nil {
			return nil, 0, realData[elementCount:]
		}
		if unsafe.Sizeof(0x0) == 8 {
			rr = int(er)
		} else {
			rr = er
		}
		return nil, rr, realData[elementCount:]

	case CloveDataTypeInt8:
		return nil, int(int8(realData[0])), realData[elementCount:]
	case CloveDataTypeUint8:
		return nil, uint(realData[0]), realData[elementCount:]
	case CloveDataTypeInt16:
		return nil, int(int16(receiver.getByteOrder().Uint16(realData))), realData[elementCount:]
	case CloveDataTypeUint16:
		return nil, uint(receiver.getByteOrder().Uint16(realData)), realData[elementCount:]
	case CloveDataTypeInt32:
		return nil, int(int32(receiver.getByteOrder().Uint32(realData))), realData[elementCount:]
	case CloveDataTypeUint32:
		return nil, uint(receiver.getByteOrder().Uint32(realData)), realData[elementCount:]
	case CloveDataTypeInt64:
		if unsafe.Sizeof(0x0) == 8 {
			return nil, int(int64(receiver.getByteOrder().Uint64(realData))), realData[elementCount:]
		} else {
			return nil, int64(receiver.getByteOrder().Uint64(realData)), realData[elementCount:]
		}
	case CloveDataTypeUint64:
		if unsafe.Sizeof(0x0) == 8 {
			return nil, uint(receiver.getByteOrder().Uint64(realData)), realData[elementCount:]
		} else {
			return nil, receiver.getByteOrder().Uint64(realData), realData[elementCount:]
		}
	case CloveDataTypeFloat32:
		return nil, math.Float32frombits(receiver.getByteOrder().Uint32(realData)), realData[elementCount:]
	case CloveDataTypeFloat64:
		return nil, math.Float64frombits(receiver.getByteOrder().Uint64(realData)), realData[elementCount:]
	case CloveDataTypeTrue:
		return nil, true, realData
	case CloveDataTypeFalse:
		return nil, false, realData
	case CloveDataTypeString:
		e := string(realData[:elementCount])
		return nil, e, realData[elementCount:]
	case CloveDataTypeBytes:
		return nil, realData[:elementCount], realData[elementCount:]
	case CloveDataTypeMap:
		var dstMap = make(CloveMap, elementCount)
		for i := 0; uint32(i) < elementCount; i++ {
			err, kd, remain := receiver.Decode(realData)
			if err != nil {
				return err, nil, nil
			}
			realData = remain
			err, vd, remain := receiver.Decode(realData)
			if err != nil {
				return err, nil, nil
			}
			realData = remain
			dstMap[kd] = vd
		}
		return nil, dstMap, realData
	case CloveDataTypeList:
		var dstList = make(CloveSlice, 0)
		for i := 0; uint32(i) < elementCount; i++ {
			err, vd, remain := receiver.Decode(realData)
			if err != nil {
				return err, nil, nil
			}
			realData = remain
			dstList = append(dstList, vd)
		}
		return nil, dstList, realData
	}
	return nil, nil, raw
}

func (receiver *EncoderClove) SetByteOrder(byteOrder binary.ByteOrder) {
	receiver.byteOrder = byteOrder
}

func (receiver EncoderClove) getByteOrder() binary.ByteOrder {
	if receiver.byteOrder == nil {
		return binary.BigEndian
	}
	return receiver.byteOrder
}
func (receiver EncoderClove) Encode(raw *CloveData) (error, []byte) {
	defer func() {
		base.LogPanic(recover())
	}()

	if *raw == nil {
		rb := make([]byte, 1)
		rb[0] = makeHeader(CloveDataTypeNil, 0)
		return nil, rb
	}
	var rawValue = reflect.ValueOf(*raw)

	return receiver.EncodeWithReflectValue(&rawValue)
}

func (receiver EncoderClove) EncodeWithReflectValue(rawValue *reflect.Value) (error, []byte) {
	defer func() {
		base.LogPanic(recover())
	}()

	fmt.Println(rawValue)

	if rawValue == nil || rawValue.IsZero() {
		rb := make([]byte, 1)
		rb[0] = makeHeader(CloveDataTypeNil, 0)
		return nil, rb
	}

	var itr = rawValue.Interface()
	var tpKind = rawValue.Type().Kind()

	var isInt = false
	var isSign = false
	var isFloat = false
	var isMap = false
	var isSlice = false
	var intRawValue int64 = 0
	var intValue uint64 = 0
	switch tpKind {
	case reflect.Int8:
		fallthrough
	case reflect.Int16:
		fallthrough
	case reflect.Int32:
		fallthrough
	case reflect.Int64:
		fallthrough
	case reflect.Int:
		intRawValue = rawValue.Int()

		intValue = uint64(intRawValue)
		isInt = true
		isSign = intRawValue < 0

	case reflect.Uint8:
		fallthrough
	case reflect.Uint16:
		fallthrough
	case reflect.Uint32:
		fallthrough
	case reflect.Uint64:
		fallthrough
	case reflect.Uint:
		intValue = uint64(rawValue.Uint())
		isInt = true
		isSign = false

	case reflect.Float32:
		isFloat = true
		isSign = false
		intValue = uint64(math.Float32bits(itr.(float32)))
	case reflect.Float64:
		isFloat = true
		isSign = false
		intValue = math.Float64bits(rawValue.Float())
	case reflect.Bool:
		rb := make([]byte, 1)
		if rawValue.Bool() {
			rb[0] = makeHeader(CloveDataTypeTrue, 0)
		} else {
			rb[0] = makeHeader(CloveDataTypeFalse, 0)
		}
		return nil, rb
	case reflect.Map:
		isMap = true
	case reflect.Slice:
		isSlice = true
	case reflect.Interface:
		x, ok := rawValue.Recv()
		if !ok {
			rb := make([]byte, 1)
			rb[0] = makeHeader(CloveDataTypeNil, 0)
			return nil, rb
		}
		return receiver.EncodeWithReflectValue(&x)
	default:
	}

	if isInt {
		//进行压缩处理
		rb := make([]byte, 9)

		if isSign {
			switch {
			case intRawValue <= 127 && intRawValue >= -128:
				rb[0] = makeHeader(CloveDataTypeInt8, 0)
				rb[1] = byte(intRawValue)
				return nil, rb[:2]
			case intRawValue <= -129 && intRawValue >= -32768:
				rb[0] = makeHeader(CloveDataTypeInt16, 0)
				receiver.getByteOrder().PutUint16(rb[1:], uint16(int16(intRawValue)))
				return nil, rb[:3]
			case intRawValue < -32768 && intRawValue >= -2147483648:
				rb[0] = makeHeader(CloveDataTypeInt32, 0)
				receiver.getByteOrder().PutUint32(rb[1:], uint32(int32(intRawValue)))
				return nil, rb[:5]
			default:
				rb[0] = makeHeader(CloveDataTypeInt64, 0)
				receiver.getByteOrder().PutUint64(rb[1:], uint64(intRawValue))
				return nil, rb
			}
		} else {
			switch {
			case intValue <= 0x7f:
				rb[0] = byte(intValue)
				return nil, rb[:1]
			case intValue <= 0xff:
				rb[0] = makeHeader(CloveDataTypeUint8, 0)
				rb[1] = byte(intValue)
				return nil, rb[:2]
			case intValue <= 0xffff:
				rb[0] = makeHeader(CloveDataTypeUint16, 0)
				receiver.getByteOrder().PutUint16(rb[1:], uint16(intValue))
				return nil, rb[:3]
			case intValue <= 0xffffffff:
				rb[0] = makeHeader(CloveDataTypeUint32, 0)
				receiver.getByteOrder().PutUint32(rb[1:], uint32(intValue))
				return nil, rb[:5]
			default:
				rb[0] = makeHeader(CloveDataTypeUint64, 0)
				receiver.getByteOrder().PutUint64(rb[1:], intValue)
				return nil, rb
			}
		}
	}

	if isFloat {
		rb := make([]byte, 9)
		if intValue <= 0xffffffff {
			rb[0] = makeHeader(CloveDataTypeFloat32, 0)
			receiver.getByteOrder().PutUint32(rb[1:], uint32(intValue))
			return nil, rb[:5]
		} else {
			rb[0] = makeHeader(CloveDataTypeFloat64, 0)
			receiver.getByteOrder().PutUint64(rb[1:], uint64(intValue))
			return nil, rb[:9]
		}
	}

	bytesRaw, isBytes := itr.([]byte)
	strRaw, isStr := itr.(string)
	if isBytes || isStr {
		rbs := make([][]byte, 2)
		rbs[1] = bytesRaw
		var etp byte = CloveDataTypeBytes
		if isStr {
			etp = CloveDataTypeString
			rbs[1] = []byte(strRaw)
		}
		rbs[0] = receiver.makeHeaderAndLength(etp, len(rbs[1]))

		return nil, bytes.Join(rbs, []byte(""))
	}

	isMap = tpKind == reflect.Map
	if isMap {
		var i = 0
		var rbs = make([][]byte, rawValue.Len()+1)
		var ks = rawValue.MapRange()
		for ks.Next() {
			var k = ks.Key().Elem()
			err, rk := receiver.EncodeWithReflectValue(&k)
			if err != nil {
				continue
			}
			var v = ks.Value().Elem()
			err, vk := receiver.EncodeWithReflectValue(&v)
			if err != nil {
				continue
			}
			rbs[i+1] = bytes.Join([][]byte{rk, vk}, []byte(""))
			i++
		}

		rbs[0] = receiver.makeHeaderAndLength(CloveDataTypeMap, i)

		return nil, bytes.Join(rbs, []byte(""))
	}

	isSlice = tpKind == reflect.Slice
	if isSlice {
		var totalLen = rawValue.Len()
		var rbs = make([][]byte, totalLen+1)
		for i := 0; i < totalLen; i++ {
			v := rawValue.Index(i).Elem()
			err, vk := receiver.EncodeWithReflectValue(&v)
			if err != nil {
				continue
			}
			rbs[i+1] = vk
		}

		rbs[0] = receiver.makeHeaderAndLength(CloveDataTypeList, totalLen)

		return nil, bytes.Join(rbs, []byte(""))
	}

	return ErrTypeNotSupported, nil
}

type IntermediateValue struct {
	raw interface{}
}

func CreateIntermediateValue(raw interface{}) *IntermediateValue {
	v := new(IntermediateValue)
	v.raw = raw
	return v
}

func (receiver IntermediateValue) String() string {
	s, ok := receiver.raw.(string)
	if !ok {
		return ""
	}
	return s
}

func (receiver IntermediateValue) Int() int {
	return receiver.IntWithDefault(0)
}

func (receiver IntermediateValue) Int64() int64 {
	return receiver.Int64WithDefault(0)
}

func (receiver IntermediateValue) IntWithDefault(def int) int {
	s, ok := receiver.raw.(int)
	if !ok {
		return def
	}
	return s
}

func (receiver IntermediateValue) Int64WithDefault(def int64) int64 {
	s, ok := receiver.raw.(int64)
	if !ok {
		return def
	}
	return s
}

func (receiver IntermediateValue) Bool() bool {
	s, ok := receiver.raw.(bool)
	if !ok {
		return false
	}
	return s
}

func (receiver IntermediateValue) Float32() float32 {
	s, ok := receiver.raw.(float32)
	if !ok {
		return 0
	}
	return s
}

func (receiver IntermediateValue) Float64() float64 {
	s, ok := receiver.raw.(float64)
	if !ok {
		return 0
	}
	return s
}

func (receiver IntermediateValue) Map() map[interface{}]interface{} {
	s, ok := receiver.raw.(map[interface{}]interface{})
	if !ok {
		return make(map[interface{}]interface{})
	}
	return s
}

func (receiver IntermediateValue) Slice() []interface{} {
	s, ok := receiver.raw.([]interface{})
	if !ok {
		return make([]interface{}, 0)
	}
	return s
}
