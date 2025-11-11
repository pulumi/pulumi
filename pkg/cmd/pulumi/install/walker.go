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

package install

import (
	"context"
	"sync"

	"golang.org/x/sync/errgroup"
)

type depWorkPool struct {
	limit errgroup.Group
	pool  sync.WaitGroup
}

func newDepWorkPool(ctx context.Context, maxParallel int) (*depWorkPool, context.Context) {
	limit, ctx := errgroup.WithContext(ctx)
	limit.SetLimit(maxParallel)
	return &depWorkPool{
		limit: *limit, //nolint:govet // We don't use the old limit again.
	}, ctx
}

func (dwp *depWorkPool) Go(f func() error, depenents ...*sync.WaitGroup) {
	dwp.pool.Add(1)
	go func() {
		for _, d := range depenents {
			d.Wait()
		}
		dwp.limit.Go(func() error {
			defer dwp.pool.Done()
			return f()
		})
	}()
}

func (dwp *depWorkPool) Wait() error {
	dwp.pool.Wait()         // Wait to ensure that all jobs are queued
	return dwp.limit.Wait() // Then wait to ensure that all jobs are completed
}
