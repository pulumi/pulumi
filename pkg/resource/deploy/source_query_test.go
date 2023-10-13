// Copyright 2016-2018, Pulumi Corporation.
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

package deploy

import (
	"context"
	"fmt"
	"testing"

	pbempty "github.com/golang/protobuf/ptypes/empty"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
)

func TestQuerySource_Trivial_Wait(t *testing.T) {
	t.Parallel()

	// Trivial querySource returns immediately with `Wait()`, even with multiple invocations.

	// Success case.
	resmon1 := mockQueryResmon{}
	qs1, _ := newTestQuerySource(&resmon1, func(*querySource) error {
		return nil
	})

	qs1.forkRun()

	err := qs1.Wait()
	assert.NoError(t, err)
	assert.False(t, resmon1.cancelled)

	err = qs1.Wait()
	assert.NoError(t, err)
	assert.False(t, resmon1.cancelled)

	// Failure case.
	resmon2 := mockQueryResmon{}
	qs2, _ := newTestQuerySource(&resmon2, func(*querySource) error {
		return fmt.Errorf("failed")
	})

	qs2.forkRun()

	err = qs2.Wait()
	assert.False(t, result.IsBail(err))
	assert.Error(t, err)
	assert.False(t, resmon2.cancelled)

	err = qs2.Wait()
	assert.False(t, result.IsBail(err))
	assert.Error(t, err)
	assert.False(t, resmon2.cancelled)
}

func TestQuerySource_Async_Wait(t *testing.T) {
	t.Parallel()

	// `Wait()` executes asynchronously.

	// Success case.
	//
	//    test blocks until querySource signals execution has started
	// -> querySource blocks until test acknowledges querySource's signal
	// -> test blocks on `Wait()` until querySource completes.
	qs1Start, qs1StartAck := make(chan interface{}), make(chan interface{})
	resmon1 := mockQueryResmon{}
	qs1, _ := newTestQuerySource(&resmon1, func(*querySource) error {
		qs1Start <- struct{}{}
		<-qs1StartAck
		return nil
	})

	qs1.forkRun()

	// Wait until querySource starts, then acknowledge starting.
	<-qs1Start
	go func() {
		qs1StartAck <- struct{}{}
	}()

	// Wait for querySource to complete.
	err := qs1.Wait()
	assert.NoError(t, err)
	assert.False(t, resmon1.cancelled)

	err = qs1.Wait()
	assert.NoError(t, err)
	assert.False(t, resmon1.cancelled)

	// Cancellation case.
	//
	//    test blocks until querySource signals execution has started
	// -> querySource blocks until test acknowledges querySource's signal
	// -> test blocks on `Wait()` until querySource completes.
	qs2Start, qs2StartAck := make(chan interface{}), make(chan interface{})
	resmon2 := mockQueryResmon{}
	qs2, cancelQs2 := newTestQuerySource(&resmon2, func(*querySource) error {
		qs2Start <- struct{}{}
		// Block forever.
		<-qs2StartAck
		return nil
	})

	qs2.forkRun()

	// Wait until querySource starts, then cancel.
	<-qs2Start
	go func() {
		cancelQs2()
	}()

	// Wait for querySource to complete.
	err = qs2.Wait()
	assert.NoError(t, err)
	assert.True(t, resmon2.cancelled)

	err = qs2.Wait()
	assert.NoError(t, err)
	assert.True(t, resmon2.cancelled)
}

func TestQueryResourceMonitor_UnsupportedOperations(t *testing.T) {
	t.Parallel()

	rm := &queryResmon{}

	_, err := rm.ReadResource(context.Background(), nil)
	assert.Error(t, err)
	assert.Equal(t, "Query mode does not support reading resources", err.Error())

	_, err = rm.RegisterResource(context.Background(), nil)
	assert.Error(t, err)
	assert.Equal(t, "Query mode does not support creating, updating, or deleting resources", err.Error())

	_, err = rm.RegisterResourceOutputs(context.Background(), nil)
	assert.Error(t, err)
	assert.Equal(t, "Query mode does not support registering resource operations", err.Error())
}

//
// Test querySoruce constructor.
//

func newTestQuerySource(mon SourceResourceMonitor,
	runLangPlugin func(*querySource) error,
) (*querySource, context.CancelFunc) {
	cancel, cancelFunc := context.WithCancel(context.Background())

	return &querySource{
		mon:               mon,
		runLangPlugin:     runLangPlugin,
		langPluginFinChan: make(chan error),
		cancel:            cancel,
	}, cancelFunc
}

//
// Mock resource monitor.
//

type mockQueryResmon struct {
	cancelled bool
}

var _ SourceResourceMonitor = (*mockQueryResmon)(nil)

func (rm *mockQueryResmon) Address() string {
	panic("not implemented")
}

func (rm *mockQueryResmon) Cancel() error {
	rm.cancelled = true
	return nil
}

func (rm *mockQueryResmon) Invoke(ctx context.Context,
	req *pulumirpc.ResourceInvokeRequest,
) (*pulumirpc.InvokeResponse, error) {
	panic("not implemented")
}

func (rm *mockQueryResmon) Call(ctx context.Context,
	req *pulumirpc.CallRequest,
) (*pulumirpc.CallResponse, error) {
	panic("not implemented")
}

func (rm *mockQueryResmon) ReadResource(ctx context.Context,
	req *pulumirpc.ReadResourceRequest,
) (*pulumirpc.ReadResourceResponse, error) {
	panic("not implemented")
}

func (rm *mockQueryResmon) RegisterResource(ctx context.Context,
	req *pulumirpc.RegisterResourceRequest,
) (*pulumirpc.RegisterResourceResponse, error) {
	panic("not implemented")
}

func (rm *mockQueryResmon) RegisterResourceOutputs(ctx context.Context,
	req *pulumirpc.RegisterResourceOutputsRequest,
) (*pbempty.Empty, error) {
	panic("not implemented")
}
