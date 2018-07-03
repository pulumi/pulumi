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

package cmdutil

import (
	"os"

	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

var snk diag.Sink

var globalColorization = colors.Always

// GetGlobalColorization gets the global setting for how things should be colored.
// This is helpful for the parts of our stack that do not take a DisplayOptions struct.
func GetGlobalColorization() colors.Colorization {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return colors.Never
	}

	return globalColorization
}

// SetGlobalColorization sets the global setting for how things should be colored.
// This is helpful for the parts of our stack that do not take a DisplayOptions struct.
func SetGlobalColorization(color colors.Colorization) {
	globalColorization = color
}

// Diag lazily allocates a sink to be used if we can't create a compiler.
func Diag() diag.Sink {
	if snk == nil {
		snk = diag.DefaultSink(os.Stdout, os.Stderr, diag.FormatOptions{
			Color: GetGlobalColorization(),
		})
	}
	return snk
}

// InitDiag forces initialization of the diagnostics sink with the given options.
func InitDiag(opts diag.FormatOptions) {
	contract.Assertf(snk == nil, "Cannot initialize diagnostics sink more than once")
	snk = diag.DefaultSink(os.Stdout, os.Stderr, opts)
}
