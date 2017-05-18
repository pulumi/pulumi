// Copyright 2017 Pulumi, Inc. All rights reserved.

package resource

import (
	"github.com/pulumi/lumi/pkg/util/mapper"
	"github.com/pulumi/lumi/sdk/go/pkg/lumirpc"
)

// NewCheckResponse produces a response with property validation failures from the given array of mapper failures.
func NewCheckResponse(err mapper.DecodeError) *lumirpc.CheckResponse {
	var checkFailures []*lumirpc.CheckFailure
	if err != nil {
		for _, failure := range err.Failures() {
			checkFailures = append(checkFailures, &lumirpc.CheckFailure{
				Property: failure.Field(),
				Reason:   failure.Reason(),
			})
		}
	}
	return &lumirpc.CheckResponse{Failures: checkFailures}
}
