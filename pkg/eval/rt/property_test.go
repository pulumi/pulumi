// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rt

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestStable ensures that property enumeration order is stable from one run to the next.
func TestStable(t *testing.T) {
	props := NewPropertyMap()

	// Seed the map.
	ks := "abcdefghijklmnopqrstuvwxyz0123456789"
	for _, k := range ks {
		props.Set(PropertyKey(string(k)), NewNullObject())
	}

	// Observe an initial key ordering.
	var observed []PropertyKey
	for _, k := range props.Stable() {
		observed = append(observed, k)
	}

	// Finally validate that the key ordering never changes (well, okay, check 100 iterations).
	for i := 0; i < 100; i++ {
		for j, k := range props.Stable() {
			assert.Equal(t, observed[j], k)
		}
	}
}

// TestChrono ensures that property enumeration order returns keys in chronological order.  Note that this will need to
// change once we adopt ECMAScript's ordering (https://tc39.github.io/ecma262/#sec-ordinaryownpropertykeys).
func TestChrono(t *testing.T) {
	props := NewPropertyMap()

	// Just add keys to the map and then ensure it enumerates chronologically.
	ks := "abcdefghijklmnopqrstuvwxyz0123456789"
	for _, k := range ks {
		props.Set(PropertyKey(string(k)), NewNullObject())
	}
	for i, k := range props.Stable() {
		assert.Equal(t, string(ks[i]), string(k))
	}
}
