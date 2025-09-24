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

var defaultMaxChunkSize = 100 * 1024 * 1024 // 100MB

// A "reference" to where a secure value is located
type secureLocationRef struct {
	container any // pointer to slice or map
	key       any // string (for map) or int (for slice)
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

func collectSecureFromKeyMap(
	objectMap map[Key]object, locationRefs *[]secureLocationRef, valuesChunks *[][]string,
) {
	for k, obj := range objectMap {
		switch value := obj.value.(type) {
		case map[Key]object:
			collectSecureFromKeyMap(value, locationRefs, valuesChunks)
		case map[string]object:
			collectSecureFromStringMap(value, locationRefs, valuesChunks)
		case []object:
			collectSecureFromArray(value, locationRefs, valuesChunks)
		case string:
			if obj.secure {
				*locationRefs = append(*locationRefs, secureLocationRef{container: objectMap, key: k})
				addStringToChunks(valuesChunks, value, defaultMaxChunkSize)
			}
			continue
		}
	}
}

func collectSecureFromStringMap(
	objectMap map[string]object, locationRefs *[]secureLocationRef, valuesChunks *[][]string,
) {
	for k, obj := range objectMap {
		switch value := obj.value.(type) {
		case map[Key]object:
			collectSecureFromKeyMap(value, locationRefs, valuesChunks)
		case map[string]object:
			collectSecureFromStringMap(value, locationRefs, valuesChunks)
		case []object:
			collectSecureFromArray(value, locationRefs, valuesChunks)
		case string:
			if obj.secure {
				*locationRefs = append(*locationRefs, secureLocationRef{container: objectMap, key: k})
				addStringToChunks(valuesChunks, value, defaultMaxChunkSize)
			}
			continue
		}
	}
}

func collectSecureFromArray(
	objectArray []object, locationRefs *[]secureLocationRef, valuesChunks *[][]string,
) {
	for i, obj := range objectArray {
		switch value := obj.value.(type) {
		case map[Key]object:
			collectSecureFromKeyMap(value, locationRefs, valuesChunks)
		case map[string]object:
			collectSecureFromStringMap(value, locationRefs, valuesChunks)
		case []object:
			collectSecureFromArray(value, locationRefs, valuesChunks)
		case string:
			if obj.secure {
				*locationRefs = append(*locationRefs, secureLocationRef{container: objectArray, key: i})
				addStringToChunks(valuesChunks, value, defaultMaxChunkSize)
			}
			continue
		}
	}
}

func collectSecureFromPlaintextKeyMap(
	plaintextMap map[Key]plaintext, locationRefs *[]secureLocationRef, valuesChunks *[][]string,
) {
	for k, pt := range plaintextMap {
		switch value := pt.value.(type) {
		case map[Key]plaintext:
			collectSecureFromPlaintextKeyMap(value, locationRefs, valuesChunks)
		case map[string]plaintext:
			collectSecureFromPlaintextStringMap(value, locationRefs, valuesChunks)
		case []plaintext:
			collectSecureFromPlaintextArray(value, locationRefs, valuesChunks)
		case string:
			if pt.secure {
				*locationRefs = append(*locationRefs, secureLocationRef{container: plaintextMap, key: k})
				addStringToChunks(valuesChunks, value, defaultMaxChunkSize)
			}
			continue
		}
	}
}

func collectSecureFromPlaintextStringMap(
	plaintextMap map[string]plaintext, locationRefs *[]secureLocationRef, valuesChunks *[][]string,
) {
	for k, pt := range plaintextMap {
		switch value := pt.value.(type) {
		case map[Key]plaintext:
			collectSecureFromPlaintextKeyMap(value, locationRefs, valuesChunks)
		case map[string]plaintext:
			collectSecureFromPlaintextStringMap(value, locationRefs, valuesChunks)
		case []plaintext:
			collectSecureFromPlaintextArray(value, locationRefs, valuesChunks)
		case string:
			if pt.secure {
				*locationRefs = append(*locationRefs, secureLocationRef{container: plaintextMap, key: k})
				addStringToChunks(valuesChunks, value, defaultMaxChunkSize)
			}
			continue
		}
	}
}

func collectSecureFromPlaintextArray(
	plaintextArray []plaintext, locationRefs *[]secureLocationRef, valuesChunks *[][]string,
) {
	for i, pt := range plaintextArray {
		switch value := pt.value.(type) {
		case map[Key]plaintext:
			collectSecureFromPlaintextKeyMap(value, locationRefs, valuesChunks)
		case map[string]plaintext:
			collectSecureFromPlaintextStringMap(value, locationRefs, valuesChunks)
		case []plaintext:
			collectSecureFromPlaintextArray(value, locationRefs, valuesChunks)
		case string:
			if pt.secure {
				*locationRefs = append(*locationRefs, secureLocationRef{container: plaintextArray, key: i})
				addStringToChunks(valuesChunks, value, defaultMaxChunkSize)
			}
			continue
		}
	}
}
