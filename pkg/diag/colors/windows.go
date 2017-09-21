// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

//+build windows

package colors

import (
	"github.com/reconquest/loreley"
)

func init() {
	loreley.Colorize = loreley.ColorizeNever
}
