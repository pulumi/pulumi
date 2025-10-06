// Copyright 2016-2025, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import "github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

// As of time of writing, the Pulumi Service batch encrypt and decrypt endpoints have a limit of 200MB per request.
// To avoid hitting this limit, we chunk secrets into pieces no larger than half this size as conservative limit.
var defaultMaxChunkSize = 100 * 1024 * 1024 // 100MB

// A "reference" to where a value is located in a container (slice or map).
type containerRef struct {
	container any // pointer to slice or map
	key       any // string (for map) or int (for slice)
}

func (r *containerRef) setObject(obj object) {
	switch container := r.container.(type) {
	case map[Key]object:
		container[r.key.(Key)] = obj
	case map[string]object:
		container[r.key.(string)] = obj
	case []object:
		container[r.key.(int)] = obj
	}
}

func (r *containerRef) setPlaintext(pt Plaintext) {
	switch container := r.container.(type) {
	case map[Key]Plaintext:
		container[r.key.(Key)] = pt
	case map[string]Plaintext:
		container[r.key.(string)] = pt
	case []Plaintext:
		container[r.key.(int)] = pt
	}
}

func collectCiphertextSecrets(objMap map[Key]object, refs *[]containerRef, ctChunks *[][]string) {
	var process func(obj object, ref containerRef)
	process = func(obj object, ref containerRef) {
		switch v := obj.value.(type) {
		case bool, int64, uint64, float64, string, nil:
			// Nothing to do
		case map[Key]object:
			for k, vv := range v {
				process(vv, containerRef{container: v, key: k})
			}
		case map[string]object:
			for k, vv := range v {
				process(vv, containerRef{container: v, key: k})
			}
		case []object:
			for i, vv := range v {
				process(vv, containerRef{container: v, key: i})
			}
		case CiphertextSecret:
			*refs = append(*refs, ref)
			addStringToChunks(ctChunks, v.value, defaultMaxChunkSize)
		default:
			contract.Failf("unexpected value of type %T", v)
		}
	}
	for k, obj := range objMap {
		process(obj, containerRef{container: objMap, key: k})
	}
}

func collectPlaintextSecrets(ptMap map[Key]Plaintext, refs *[]containerRef, ptChunks *[][]string) {
	var process func(pt Plaintext, ref containerRef)
	process = func(pt Plaintext, ref containerRef) {
		switch v := pt.value.(type) {
		case bool, int64, uint64, float64, string, nil:
			// Nothing to do
		case map[Key]Plaintext:
			for k, vv := range v {
				process(vv, containerRef{container: v, key: k})
			}
		case map[string]Plaintext:
			for k, vv := range v {
				process(vv, containerRef{container: v, key: k})
			}
		case []Plaintext:
			for i, vv := range v {
				process(vv, containerRef{container: v, key: i})
			}
		case PlaintextSecret:
			*refs = append(*refs, ref)
			addStringToChunks(ptChunks, string(v), defaultMaxChunkSize)
		default:
			contract.Failf("unexpected value of type %T", v)
		}
	}
	for k, pt := range ptMap {
		process(pt, containerRef{container: ptMap, key: k})
	}
}

// addStringToChunks adds a value to the last values chunk or creates a new chunk if needed.
func addStringToChunks(valuesChunks *[][]string, value string, maxChunkSize int) {
	valueLength := len(value)
	if len(*valuesChunks) == 0 {
		*valuesChunks = append(*valuesChunks, []string{value})
		return
	}
	lastChunk := &(*valuesChunks)[len(*valuesChunks)-1]
	currentSize := 0
	for _, lastChunkValue := range *lastChunk {
		currentSize += len(lastChunkValue)
	}
	if currentSize+valueLength <= maxChunkSize {
		*lastChunk = append(*lastChunk, value)
	} else {
		*valuesChunks = append(*valuesChunks, []string{value})
	}
}
