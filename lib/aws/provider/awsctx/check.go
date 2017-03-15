// Copyright 2017 Pulumi, Inc. All rights reserved.

package awsctx

import (
	"github.com/pulumi/coconut/pkg/util/mapper"
	"github.com/pulumi/coconut/sdk/go/pkg/cocorpc"
)

// NewCheckResponse produces a response with property validation failures from the given array of mapper failures.
func NewCheckResponse(err mapper.DecodeError) *cocorpc.CheckResponse {
	var checkFailures []*cocorpc.CheckFailure
	if err != nil {
		for _, failure := range err.Failures() {
			checkFailures = append(checkFailures, &cocorpc.CheckFailure{
				Property: failure.Field(),
				Reason:   failure.Reason(),
			})
		}
	}
	return &cocorpc.CheckResponse{Failures: checkFailures}
}
