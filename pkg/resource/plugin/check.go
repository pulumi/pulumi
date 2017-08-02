// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package plugin

import (
	"github.com/pulumi/pulumi-fabric/pkg/util/mapper"
	"github.com/pulumi/pulumi-fabric/sdk/go/pkg/lumirpc"
)

// NewCheckResponse produces a response with property validation failures from the given array of mapper failures.
func NewCheckResponse(err error) *lumirpc.CheckResponse {
	var failures []*lumirpc.CheckFailure
	if err != nil {
		switch e := err.(type) {
		case mapper.MappingError:
			for _, failure := range e.Failures() {
				switch f := failure.(type) {
				case mapper.FieldError:
					failures = append(failures, &lumirpc.CheckFailure{
						Property: f.Field(),
						Reason:   f.Reason(),
					})
				default:
					failures = append(failures, &lumirpc.CheckFailure{Reason: f.Error()})
				}
			}
		default:
			failures = append(failures, &lumirpc.CheckFailure{Reason: e.Error()})
		}
	}
	return &lumirpc.CheckResponse{Failures: failures}
}
