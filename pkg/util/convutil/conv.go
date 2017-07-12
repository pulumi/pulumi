// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package convutil

func Int64PToFloat64P(v *int64) *float64 {
	if v == nil {
		return nil
	}
	cv := float64(*v)
	return &cv
}

func Float64PToInt64P(v *float64) *int64 {
	if v == nil {
		return nil
	}
	cv := int64(*v)
	return &cv
}
