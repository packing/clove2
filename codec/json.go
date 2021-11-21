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

// @Title           json.go
// @Description     Realize serialization and deserialization of JSON data and Golang data.
// @Author          starlove
package codec

import (
	"encoding/binary"
	"encoding/json"
)

// DecoderJSONv1	Json data deserialization class
type DecoderJSONv1 struct {
}

// EncoderJSONv1	Json data serialization class
type EncoderJSONv1 struct {
}

var JsonCodec = Codec{Protocol: ProtocolJSON, Version: 1, Decoder: new(DecoderJSONv1), Encoder: new(EncoderJSONv1), Name: "JSON"}

// @title           tsMap
// @description     Convert the dictionary of string keys in Map to the dictionary of interface{} keys.
// @auth            starlove
// @param           m        map[string]interface{}         "Golang map-data deserialized from JSON"
func tsMap(m map[string]interface{}) CloveMap {
	rm := make(CloveMap)
	for k, v := range m {
		switch v.(type) {
		case map[string]interface{}:
			rm[k] = tsMap(v.(map[string]interface{}))
		case []interface{}:
			rm[k] = tsSlice(v.([]interface{}))
		default:
			rm[k] = v
		}
	}
	return rm
}

// @title           tsSlice
// @description     Convert the dictionary of string keys in Slice to the dictionary of interface{} keys
// @auth            starlove
// @param           s        []interface{}         "Golang slice-data deserialized from JSON"
func tsSlice(s []interface{}) CloveSlice {
	rs := make(CloveSlice, len(s))
	for i, v := range s {
		switch v.(type) {
		case map[string]interface{}:
			rs[i] = tsMap(v.(map[string]interface{}))
		case []interface{}:
			rs[i] = tsSlice(v.([]interface{}))
		default:
			rs[i] = v
		}
	}
	return rs
}

// @title           SetByteOrder
// @description     Just to implement a meaningless function of interface "Decoder".
// @auth            starlove
// @param           byteOrder    binary.ByteOrder	"Parameters that do not need to be passed"
func (receiver *DecoderJSONv1) SetByteOrder(byteOrder binary.ByteOrder) {}

// @title           Decode
// @description     Deserialize from JSON string data to Clove data.
// @auth            starlove
// @param           raw 	[]byte	"JSON string data"
// @return			error 		"Errors occurred during the process"
// @return			CloveData 	"A complete Clove data decoded (if there are no errors)"
// @return			[]byte 		"Remaining JSON string data (if the incoming JSON string data contains more than one data body)"
func (receiver DecoderJSONv1) Decode(raw []byte) (error, CloveData, []byte) {
	dst := make(map[string]interface{})
	err := json.Unmarshal(raw, &dst)
	if err != nil && err.Error() == "json: cannot unmarshal array into Go value of type map[string]interface {}" {
		dst := make([]interface{}, 0)
		err = json.Unmarshal(raw, &dst)
		return err, tsSlice(dst), []byte("")
	} else {
		return err, tsMap(dst), []byte("")
	}
}

// @title           SetByteOrder
// @description     Just to implement a meaningless function of interface "Decoder".
// @auth            starlove
// @param           byteOrder    binary.ByteOrder	"Parameters that do not need to be passed"
func (receiver *EncoderJSONv1) SetByteOrder(binary.ByteOrder) {}

// @title           Encode
// @description     Serialize from Clove data to JSON string data.
// @auth            starlove
// @param           raw 	*CloveData	"Pointer of Clove data"
// @return			error 		"Errors occurred during the process"
// @return			[]byte 		"Final JSON string data (if there are no errors)"
func (receiver EncoderJSONv1) Encode(raw *CloveData) (error, []byte) {
	bs, err := json.Marshal(raw)
	return err, bs
}
