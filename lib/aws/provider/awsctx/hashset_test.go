// Copyright 2016-2017, Pulumi Corporation
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

package awsctx

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type Setting struct {
	Namespace string
	Name      string
	Value     string
}

var _ Hashable = Setting{}

func (s Setting) HashKey() Hash {
	return Hash(s.Namespace + ":" + s.Name)
}
func (s Setting) HashValue() Hash {
	return Hash(s.Namespace + ":" + s.Name + ":" + s.Value)
}
func NewSettingHashSet(options *[]Setting) *HashSet {
	set := NewHashSet()
	if options == nil {
		return set
	}
	for _, option := range *options {
		set.Add(option)
	}
	return set
}

func TestHashSetSimple(t *testing.T) {
	items := &[]Setting{
		{
			Namespace: "a",
			Name:      "b",
			Value:     "x",
		},
		{
			Namespace: "a",
			Name:      "c",
			Value:     "y",
		},
	}

	set := NewSettingHashSet(items)
	assert.Equal(t, 2, set.Length(), "expected length is 2")
}

func TestHashSetConflicts(t *testing.T) {
	items := &[]Setting{
		{
			Namespace: "a",
			Name:      "b",
			Value:     "x",
		},
		{
			Namespace: "a",
			Name:      "b",
			Value:     "y",
		},
	}

	set := NewSettingHashSet(items)
	assert.Equal(t, 1, set.Length(), "expected length is 1")
}

func TestHashSetDiffReorder(t *testing.T) {
	itemsOld := &[]Setting{
		{
			Namespace: "a",
			Name:      "b",
			Value:     "x",
		},
		{
			Namespace: "a",
			Name:      "c",
			Value:     "y",
		},
	}
	itemsNew := &[]Setting{
		{
			Namespace: "a",
			Name:      "c",
			Value:     "y",
		},
		{
			Namespace: "a",
			Name:      "b",
			Value:     "x",
		},
	}

	oldSet := NewSettingHashSet(itemsOld)
	newSet := NewSettingHashSet(itemsNew)
	d := oldSet.Diff(newSet)
	assert.Equal(t, 0, len(d.Deletes()), "expected no deletes")
	assert.Equal(t, 0, len(d.Adds()), "expected no adds")
	assert.Equal(t, 0, len(d.Updates()), "expected no updates")
}

func TestHashSetDiffUpdate(t *testing.T) {
	itemsOld := &[]Setting{
		{
			Namespace: "a",
			Name:      "b",
			Value:     "x",
		},
		{
			Namespace: "a",
			Name:      "c",
			Value:     "y",
		},
	}
	itemsNew := &[]Setting{
		{
			Namespace: "a",
			Name:      "c",
			Value:     "z",
		},
		{
			Namespace: "a",
			Name:      "b",
			Value:     "x",
		},
	}

	oldSet := NewSettingHashSet(itemsOld)
	newSet := NewSettingHashSet(itemsNew)
	d := oldSet.Diff(newSet)
	assert.Equal(t, 0, len(d.Deletes()), "expected no deletes")
	assert.Equal(t, 0, len(d.Adds()), "expected no adds")
	assert.Equal(t, 1, len(d.Updates()), "expected 1 update")
}

func TestHashSetDiffUpdateDeleteAndAdd(t *testing.T) {
	itemsOld := &[]Setting{
		{ // this is deleted
			Namespace: "a",
			Name:      "b",
			Value:     "x",
		},
		{ // this is updated
			Namespace: "a",
			Name:      "c",
			Value:     "y",
		},
	}
	itemsNew := &[]Setting{
		{
			Namespace: "a",
			Name:      "c",
			Value:     "z",
		},
		{ // This is added
			Namespace: "b",
			Name:      "a",
			Value:     "x",
		},
	}

	oldSet := NewSettingHashSet(itemsOld)
	newSet := NewSettingHashSet(itemsNew)
	d := oldSet.Diff(newSet)
	assert.Equal(t, 1, len(d.Deletes()), "expected 1 delete")
	assert.Equal(t, 1, len(d.Adds()), "expected 1 add1")
	assert.Equal(t, 1, len(d.Updates()), "expected 1 update")
}
