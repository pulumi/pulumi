package joincontext

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
)

func withDeadline(deadline time.Time) context.Context {
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	contract.Ignore(cancel)
	return ctx
}

func withValue(k, v interface{}) context.Context {
	return context.WithValue(context.Background(), k, v)
}

func testJoin(t *testing.T, cancelled, deadlineExceeded int, joinCtx context.Context) {
	done := make(chan struct{})
	if cancelled < 0 && deadlineExceeded < 0 {
		go func() {
			time.Sleep(50 * time.Millisecond)
			close(done)
		}()
	}

	join := joinCtx.(*joinContext)
	select {
	case <-done:
		// OK
	case <-join.Done():
		assert.True(t, cancelled >= 0 || deadlineExceeded >= 0)
		if cancelled >= 0 {
			assert.Equal(t, context.Canceled, join.Err())
			assert.Equal(t, join.contexts[cancelled].Err(), join.Err())
		}
		if deadlineExceeded >= 0 {
			assert.Equal(t, context.DeadlineExceeded, join.Err())
			assert.Equal(t, join.contexts[deadlineExceeded].Err(), join.Err())
		}
	}
}

func TestSingle(t *testing.T) {
	ctx := context.Background()
	join := Join(ctx)
	assert.Equal(t, ctx, join)
}

func TestDeadline_None(t *testing.T) {
	join := Join(context.Background(), context.Background())
	_, ok := join.Deadline()
	assert.False(t, ok)

	testJoin(t, -1, -1, join)
}

func TestDeadline_Single(t *testing.T) {
	expected := time.Now().Add(1 * time.Second)
	join := Join(context.Background(), withDeadline(expected))
	deadline, ok := join.Deadline()
	assert.True(t, ok)
	assert.Equal(t, expected, deadline)

	testJoin(t, -1, 1, join)
}

func TestDeadline_Soonest(t *testing.T) {
	expected := time.Now().Add(1 * time.Second)
	join := Join(context.Background(), withDeadline(expected), withDeadline(expected.Add(10*time.Second)))
	deadline, ok := join.Deadline()
	assert.True(t, ok)
	assert.Equal(t, expected, deadline)

	testJoin(t, -1, 1, join)
}

func TestCancel_2(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	join := Join(context.Background(), cancelled)
	testJoin(t, 1, -1, join)
}

func TestCancel_3(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	join := Join(context.Background(), context.Background(), cancelled)
	testJoin(t, 2, -1, join)
}

func TestValue_None(t *testing.T) {
	join := Join(context.Background(), context.Background())
	val := join.Value(42)
	assert.Nil(t, val)
}

func TestValue_Single(t *testing.T) {
	join := Join(context.Background(), withValue(42, 24))
	val := join.Value(42)
	assert.Equal(t, 24, val)
}

func TestValue_First(t *testing.T) {
	join := Join(context.Background(), withValue(42, 24), withValue(42, 84))
	val := join.Value(42)
	assert.Equal(t, 24, val)
}
