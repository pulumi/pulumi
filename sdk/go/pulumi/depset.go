// Copyright 2024, Pulumi Corporation.
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

package pulumi

import (
	"sort"

	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
)

type depSet map[URN]Resource

func (d depSet) add(v URN, r Resource) {
	d[v] = r
}

func (d depSet) urns() []URN {
	values := slice.Prealloc[URN](len(d))
	for v := range d {
		values = append(values, v)
	}
	return values
}

func (d depSet) sortedURNs() []URN {
	v := d.urns()
	sort.Slice(v, func(i, j int) bool { return v[i] < v[j] })
	return v
}

func (d depSet) toURNSet() urnSet {
	s := make(urnSet)
	for v := range d {
		s.add(v)
	}
	return s
}
