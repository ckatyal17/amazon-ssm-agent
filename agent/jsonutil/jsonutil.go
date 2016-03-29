// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Amazon Software License (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/asl/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package jsonutil contains various utilities for dealing with json data.
package jsonutil

import (
	"bytes"
	"encoding/json"
)

// Indent indents a json string.
func Indent(jsonStr string) string {
	var dst bytes.Buffer
	json.Indent(&dst, []byte(jsonStr), "", "  ")
	return string(dst.Bytes())
}

// Remarshal marshals an object to Json then parses it back to another object.
// This is useful for example when we want to go from map[string]interface{}
// to a more specific struct type or if we want a deep copy of the object.
func Remarshal(obj interface{}, remarshalledObj interface{}) (err error) {
	b, err := json.Marshal(obj)
	if err != nil {
		return
	}
	err = json.Unmarshal(b, remarshalledObj)
	if err != nil {
		return
	}
	return nil
}

// Marshal marshals an object to a json string.
// Returns empty string if marshal fails.
func Marshal(obj interface{}) (result string, err error) {
	var resultB []byte
	resultB, err = json.Marshal(obj)
	if err != nil {
		return
	}
	result = string(resultB)
	return
}

// UnmarshalFile reads the content of a file then Unmarshals the content to an object.
func UnmarshalFile(filePath string, dest interface{}) (err error) {
	content, err := ioUtil.ReadFile(filePath)
	if err != nil {
		return
	}
	err = json.Unmarshal(content, dest)
	return
}
