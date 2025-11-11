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
		limit: *limit, //nolint:copylocks // We don't use the old limit again.
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
