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
	"errors"
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
	var called1 bool
	resmon1 := mockResmon{
		CancelF: func() error {
			called1 = true
			return nil
		},
	}
	qs1, _ := newTestQuerySource(&resmon1, func(*querySource) error {
		return nil
	})

	qs1.forkRun()

	err := qs1.Wait()
	assert.NoError(t, err)
	assert.False(t, called1)

	// Can be called twice.
	err = qs1.Wait()
	assert.NoError(t, err)

	// Failure case.
	var called2 bool
	resmon2 := mockResmon{
		CancelF: func() error {
			called2 = true
			return nil
		},
	}
	qs2, _ := newTestQuerySource(&resmon2, func(*querySource) error {
		return errors.New("failed")
	})

	qs2.forkRun()

	err = qs2.Wait()
	assert.False(t, result.IsBail(err))
	assert.Error(t, err)
	assert.False(t, called2)

	// Can be called twice.
	err = qs2.Wait()
	assert.False(t, result.IsBail(err))
	assert.Error(t, err)
	assert.False(t, called2)
}

func TestQuerySource_Async_Wait(t *testing.T) {
	t.Parallel()

	// `Wait()` executes asynchronously.

	// Success case.
	//
	//    test blocks until querySource signals execution has started
	// -> querySource blocks until test acknowledges querySource's signal
	// -> test blocks on `Wait()` until querySource completes.
	var called1 bool
	resmon1 := mockResmon{
		CancelF: func() error {
			called1 = true
			return nil
		},
	}
	qs1Start, qs1StartAck := make(chan interface{}), make(chan interface{})
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
	assert.False(t, called1)

	err = qs1.Wait()
	assert.NoError(t, err)
	assert.False(t, called1)

	var called2 bool
	resmon2 := mockResmon{
		CancelF: func() error {
			called2 = true
			return nil
		},
	}
	// Cancellation case.
	//
	//    test blocks until querySource signals execution has started
	// -> querySource blocks until test acknowledges querySource's signal
	// -> test blocks on `Wait()` until querySource completes.
	qs2Start, qs2StartAck := make(chan interface{}), make(chan interface{})
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
	assert.True(t, called2)

	err = qs2.Wait()
	assert.NoError(t, err)
	assert.True(t, called2)
}

func TestQueryResourceMonitor_UnsupportedOperations(t *testing.T) {
	t.Parallel()

	rm := &queryResmon{}

	_, err := rm.ReadResource(context.Background(), nil)
	assert.EqualError(t, err, "Query mode does not support reading resources")

	_, err = rm.RegisterResource(context.Background(), nil)
	assert.EqualError(t, err, "Query mode does not support creating, updating, or deleting resources")

	_, err = rm.RegisterResourceOutputs(context.Background(), nil)
	assert.EqualError(t, err, "Query mode does not support registering resource operations")
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

type mockResmon struct {
	AddressF func() string

	CancelF func() error

	InvokeF func(ctx context.Context,
		req *pulumirpc.ResourceInvokeRequest) (*pulumirpc.InvokeResponse, error)

	CallF func(ctx context.Context,
		req *pulumirpc.CallRequest) (*pulumirpc.CallResponse, error)

	ReadResourceF func(ctx context.Context,
		req *pulumirpc.ReadResourceRequest) (*pulumirpc.ReadResourceResponse, error)

	RegisterResourceF func(ctx context.Context,
		req *pulumirpc.RegisterResourceRequest) (*pulumirpc.RegisterResourceResponse, error)

	RegisterResourceOutputsF func(ctx context.Context,
		req *pulumirpc.RegisterResourceOutputsRequest) (*pbempty.Empty, error)
}

var _ SourceResourceMonitor = (*mockResmon)(nil)

func (rm *mockResmon) Address() string {
	if rm.AddressF != nil {
		return rm.AddressF()
	}
	panic("not implemented")
}

func (rm *mockResmon) Cancel() error {
	if rm.CancelF != nil {
		return rm.CancelF()
	}
	panic("not implemented")
}

func (rm *mockResmon) Invoke(ctx context.Context,
	req *pulumirpc.ResourceInvokeRequest,
) (*pulumirpc.InvokeResponse, error) {
	if rm.InvokeF != nil {
		return rm.InvokeF(ctx, req)
	}
	panic("not implemented")
}

func (rm *mockResmon) Call(ctx context.Context,
	req *pulumirpc.CallRequest,
) (*pulumirpc.CallResponse, error) {
	if rm.CallF != nil {
		return rm.CallF(ctx, req)
	}
	panic("not implemented")
}

func (rm *mockResmon) ReadResource(ctx context.Context,
	req *pulumirpc.ReadResourceRequest,
) (*pulumirpc.ReadResourceResponse, error) {
	if rm.ReadResourceF != nil {
		return rm.ReadResourceF(ctx, req)
	}
	panic("not implemented")
}

func (rm *mockResmon) RegisterResource(ctx context.Context,
	req *pulumirpc.RegisterResourceRequest,
) (*pulumirpc.RegisterResourceResponse, error) {
	if rm.RegisterResourceF != nil {
		return rm.RegisterResourceF(ctx, req)
	}
	panic("not implemented")
}

func (rm *mockResmon) RegisterResourceOutputs(ctx context.Context,
	req *pulumirpc.RegisterResourceOutputsRequest,
) (*pbempty.Empty, error) {
	if rm.RegisterResourceOutputsF != nil {
		return rm.RegisterResourceOutputsF(ctx, req)
	}
	panic("not implemented")
}
