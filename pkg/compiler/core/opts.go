// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package core

import (
	"github.com/pulumi/lumi/pkg/diag"
)

// Options contains all of the settings a user can use to control the compiler's behavior.
type Options struct {
	Diag diag.Sink // a sink to use for all diagnostics.
}

// DefaultOptions returns the default set of compiler options.
func DefaultOptions() *Options {
	return &Options{}
}

// DefaultSink returns the default preconfigured diagnostics sink.
func DefaultSink(path string) diag.Sink {
	return diag.DefaultSink(diag.FormatOptions{
		Pwd:    path, // ensure output paths are relative to the current path.
		Colors: true, // turn on colorization of warnings/errors.
	})
}
