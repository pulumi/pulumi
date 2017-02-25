// Copyright 2016 Pulumi, Inc. All rights reserved.

package awsctx

import (
	"time"

	"github.com/pulumi/coconut/pkg/util/retry"
)

const DefaultTimeout time.Duration = 30 * time.Second    // by default, wait at most 30 seconds for things.
const DefaultTimeoutLong time.Duration = 5 * time.Minute // for really long operations, wait at most 5 minutes.

// RetryUntil is a thin wrapper atop util.retry.Until that uses standard retry and backoff values for AWS operations.
func RetryUntil(ctx *Context, acc retry.Acceptance) (bool, error) {
	return retry.UntilTimeout(ctx.Request(), retry.Acceptor{
		Accept: acc,
	}, DefaultTimeout)
}

// RetryUntilLong is a thin wrapper atop util.retry.Until that uses standard retry and backoff values for AWS operations.
func RetryUntilLong(ctx *Context, acc retry.Acceptance) (bool, error) {
	return retry.UntilTimeout(ctx.Request(), retry.Acceptor{
		Accept: acc,
	}, DefaultTimeoutLong)
}
