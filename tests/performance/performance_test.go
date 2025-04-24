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

//go:build all

package perf

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
)

// TODO: add tests using other languages https://github.com/pulumi/pulumi/issues/17669

//nolint:paralleltest // Do not run in parallel to avoid resource contention
func TestPerfEmptyUpdate(t *testing.T) {
	benchmarkEnforcer := &integration.AssertPerfBenchmark{
		T:                  t,
		MaxPreviewDuration: 6300 * time.Millisecond,
		MaxUpdateDuration:  6300 * time.Millisecond,
	}

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		NoParallel: true,
		Dir:        filepath.Join("python", "empty"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python"),
		},
		Quick:       true,
		ReportStats: benchmarkEnforcer,
		CloudURL:    integration.MakeTempBackend(t),
	})
}

//nolint:paralleltest // Do not run in parallel to avoid resource contention
func TestPerfManyComponentUpdate(t *testing.T) {
	benchmarkEnforcer := &integration.AssertPerfBenchmark{
		T:                  t,
		MaxPreviewDuration: 18100 * time.Millisecond,
		MaxUpdateDuration:  18100 * time.Millisecond,
	}

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		NoParallel: true,
		Dir:        filepath.Join("python", "component"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python"),
		},
		Quick:       true,
		ReportStats: benchmarkEnforcer,
		CloudURL:    integration.MakeTempBackend(t),
		LocalProviders: []integration.LocalDependency{
			{Package: "testprovider", Path: filepath.Join("..", "testprovider")},
		},
	})
}

//nolint:paralleltest // Do not run in parallel to avoid resource contention
func TestPerfParentChainUpdate(t *testing.T) {
	benchmarkEnforcer := &integration.AssertPerfBenchmark{
		T:                  t,
		MaxPreviewDuration: 19300 * time.Millisecond,
		MaxUpdateDuration:  19300 * time.Millisecond,
	}

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		NoParallel: true,
		Dir:        filepath.Join("python", "parents"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python"),
		},
		Quick:       true,
		ReportStats: benchmarkEnforcer,
		CloudURL:    integration.MakeTempBackend(t),
		LocalProviders: []integration.LocalDependency{
			{Package: "testprovider", Path: filepath.Join("..", "testprovider")},
		},
	})
}

//nolint:paralleltest // Do not run in parallel to avoid resource contention
func TestPerfSecretsBatchUpdate(t *testing.T) {
	benchmarkEnforcer := &integration.AssertPerfBenchmark{
		T:                  t,
		MaxPreviewDuration: 5 * time.Second,
		MaxUpdateDuration:  5 * time.Second,
	}

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		NoParallel: true,
		Dir:        filepath.Join("python", "secrets"),
		Dependencies: []string{
			filepath.Join("..", "..", "sdk", "python"),
		},
		Quick:          false,
		RequireService: true,
		ReportStats:    benchmarkEnforcer,
	})
}
