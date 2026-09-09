// Copyright 2026, Pulumi Corporation.
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

package pulumi

// These helpers are defined in a file separate from TestSourcePosition so that the test
// can assert the recorded source position points at the registration call site (in this
// file) rather than one frame too high (the caller in context_test.go). Keeping them in a
// different file makes an off-by-one frame error observable via the file name in the
// source position URI, which a same-file closure would hide.

func registerSourcePositionResource(ctx *Context) error {
	var res testResource2
	return ctx.RegisterResource("test:resource:type", "reg", &testResource2Inputs{}, &res)
}

func readSourcePositionResource(ctx *Context) error {
	var res testResource2
	return ctx.ReadResource("test:resource:type", "read", ID("myid"), &testResource2Inputs{}, &res)
}
