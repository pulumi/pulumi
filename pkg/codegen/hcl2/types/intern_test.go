// Copyright 2026, Pulumi Corporation.
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

package types

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tableSizes() (int, int) {
	nodeTable.mu.Lock()
	defer nodeTable.mu.Unlock()
	cellTable.mu.Lock()
	defer cellTable.mu.Unlock()
	return len(nodeTable.m), len(cellTable.m)
}

// TestInternTablesRelease checks that the intern tables drop their entries once no type refers to them, and that a
// type that is still referenced keeps its identity across collections.
func TestInternTablesRelease(t *testing.T) { //nolint:paralleltest // Counts the entries of the package tables.
	nodesBefore, cellsBefore := tableSizes()

	keep := Object(map[string]Type{"keep": List(Int)})
	build := func(i int) Type {
		return Object(map[string]Type{fmt.Sprintf("p%d", i): List(Union(String, Const(int64(i))))})
	}
	for i := range 2000 {
		build(i)
	}
	nodesFull, cellsFull := tableSizes()
	require.Greater(t, nodesFull, nodesBefore+2000)
	require.Greater(t, cellsFull, cellsBefore+2000)

	// Cleanups run on their own goroutines after a collection, so poll for the tables to drain.
	var nodesAfter, cellsAfter int
	for range 200 {
		runtime.GC()
		time.Sleep(time.Millisecond)
		if nodesAfter, cellsAfter = tableSizes(); nodesAfter <= nodesBefore+8 && cellsAfter <= cellsBefore+8 {
			break
		}
	}
	assert.LessOrEqual(t, nodesAfter, nodesBefore+8, "node table did not drain")
	assert.LessOrEqual(t, cellsAfter, cellsBefore+8, "cell table did not drain")

	// The kept type and a fresh construction of it are still one entry.
	assert.Equal(t, keep, Object(map[string]Type{"keep": List(Int)}))
	runtime.KeepAlive(keep)
}
