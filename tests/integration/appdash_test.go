// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.
//
// Utilities for testing AppDash-based tracing files.

package ints

import (
	"os"

	"sourcegraph.com/sourcegraph/appdash"
)

func ReadMemoryStoreFromFile(file string) (*appdash.MemoryStore, error) {
	store := appdash.NewMemoryStore()

	traceFile, err := os.Open(file)
	defer traceFile.Close()

	if err != nil {
		return nil, err
	}

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
