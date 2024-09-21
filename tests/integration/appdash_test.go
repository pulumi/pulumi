// Copyright 2021-2024, Pulumi Corporation.
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

// Utilities for testing AppDash-based tracing files.

package ints

import (
	"os"

	"github.com/pulumi/appdash"
)

func ReadMemoryStoreFromFile(file string) (*appdash.MemoryStore, error) {
	store := appdash.NewMemoryStore()

	traceFile, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer traceFile.Close()

	_, err = store.ReadFrom(traceFile)
	if err != nil {
		return nil, err
	}

	return store, nil
}

func WalkTracesWithDescendants(store *appdash.MemoryStore, visit func(t *appdash.Trace) error) error {
	traces, err := store.Traces(appdash.TracesOpts{})
	if err != nil {
		return err
	}

	var recur func(t *appdash.Trace) error
	recur = func(t *appdash.Trace) error {
		err := visit(t)
		if err != nil {
			return err
		}

		for _, s := range t.Sub {
			err = recur(s)
			if err != nil {
				return err
			}
		}

		return nil
	}

	for _, t := range traces {
		err := recur(t)
		if err != nil {
			return err
		}
	}

	return nil
}

func FindTrace(store *appdash.MemoryStore, matching func(tr *appdash.Trace) bool) (*appdash.Trace, error) {
	var trace *appdash.Trace
	err := WalkTracesWithDescendants(store, func(tr *appdash.Trace) error {
		if matching(tr) {
			trace = tr
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return trace, nil
}
