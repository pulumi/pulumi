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

func collectCiphertextSecretsFromKeyMap(
	objMap map[Key]object, refs *[]containerRef, ctChunks *[][]string,
) {
	for k, obj := range objMap {
		switch value := obj.value.(type) {
		case map[Key]object:
			collectCiphertextSecretsFromKeyMap(value, refs, ctChunks)
		case map[string]object:
			collectCiphertextSecretsFromStringMap(value, refs, ctChunks)
		case []object:
			collectCiphertextSecretsFromArray(value, refs, ctChunks)
		case CiphertextSecret:
			*refs = append(*refs, containerRef{container: objMap, key: k})
			addStringToChunks(ctChunks, value.value, defaultMaxChunkSize)
		}
	}
}

func collectCiphertextSecretsFromStringMap(
	objMap map[string]object, refs *[]containerRef, ctChunks *[][]string,
) {
	for k, obj := range objMap {
		switch value := obj.value.(type) {
		case map[Key]object:
			collectCiphertextSecretsFromKeyMap(value, refs, ctChunks)
		case map[string]object:
			collectCiphertextSecretsFromStringMap(value, refs, ctChunks)
		case []object:
			collectCiphertextSecretsFromArray(value, refs, ctChunks)
		case CiphertextSecret:
			*refs = append(*refs, containerRef{container: objMap, key: k})
			addStringToChunks(ctChunks, value.value, defaultMaxChunkSize)
		}
	}
}

func collectCiphertextSecretsFromArray(
	objArray []object, refs *[]containerRef, ctChunks *[][]string,
) {
	for i, obj := range objArray {
		switch value := obj.value.(type) {
		case map[Key]object:
			collectCiphertextSecretsFromKeyMap(value, refs, ctChunks)
		case map[string]object:
			collectCiphertextSecretsFromStringMap(value, refs, ctChunks)
		case []object:
			collectCiphertextSecretsFromArray(value, refs, ctChunks)
		case CiphertextSecret:
			*refs = append(*refs, containerRef{container: objArray, key: i})
			addStringToChunks(ctChunks, value.value, defaultMaxChunkSize)
		}
	}
}

func collectPlaintextSecretsFromKeyMap(
	ptMap map[Key]Plaintext, refs *[]containerRef, ptChunks *[][]string,
) {
	for k, pt := range ptMap {
		switch value := pt.value.(type) {
		case map[Key]Plaintext:
			collectPlaintextSecretsFromKeyMap(value, refs, ptChunks)
		case map[string]Plaintext:
			collectPlaintextSecretsFromStringMap(value, refs, ptChunks)
		case []Plaintext:
			collectPlaintextSecretsFromArray(value, refs, ptChunks)
		case PlaintextSecret:
			*refs = append(*refs, containerRef{container: ptMap, key: k})
			addStringToChunks(ptChunks, string(value), defaultMaxChunkSize)
		}
	}
}

func collectPlaintextSecretsFromStringMap(
	ptMap map[string]Plaintext, refs *[]containerRef, ptChunks *[][]string,
) {
	for k, pt := range ptMap {
		switch value := pt.value.(type) {
		case map[Key]Plaintext:
			collectPlaintextSecretsFromKeyMap(value, refs, ptChunks)
		case map[string]Plaintext:
			collectPlaintextSecretsFromStringMap(value, refs, ptChunks)
		case []Plaintext:
			collectPlaintextSecretsFromArray(value, refs, ptChunks)
		case PlaintextSecret:
			*refs = append(*refs, containerRef{container: ptMap, key: k})
			addStringToChunks(ptChunks, string(value), defaultMaxChunkSize)
		}
	}
}

func collectPlaintextSecretsFromArray(
	ptArray []Plaintext, refs *[]containerRef, ptChunks *[][]string,
) {
	for i, pt := range ptArray {
		switch value := pt.value.(type) {
		case map[Key]Plaintext:
			collectPlaintextSecretsFromKeyMap(value, refs, ptChunks)
		case map[string]Plaintext:
			collectPlaintextSecretsFromStringMap(value, refs, ptChunks)
		case []Plaintext:
			collectPlaintextSecretsFromArray(value, refs, ptChunks)
		case PlaintextSecret:
			*refs = append(*refs, containerRef{container: ptArray, key: i})
			addStringToChunks(ptChunks, string(value), defaultMaxChunkSize)
		}
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
