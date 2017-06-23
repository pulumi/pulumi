// Copyright 2016-2017, Pulumi Corporation
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

package errors

// Eval errors are in the [1000,2000) range.
var (
	ErrorUnhandledException        = newError(1000, "An unhandled exception terminated the program: %v")
	ErrorUnhandledInitException    = newError(1001, "An unhandled exception in %v's initializer occurred: %v")
	ErrorPackageHasNoDefaultModule = newError(1002, "Package '%v' is missing a default module")
	ErrorModuleHasNoEntryPoint     = newError(1003, "Module '%v' is missing an entrypoint function")
	ErrorFunctionArgMismatch       = newError(1004, "Function expected %v arguments, but only got %v")
	ErrorFunctionArgIncorrectType  = newError(1005, "Function argument has an incorrect type; expected %v, got %v")
	ErrorFunctionArgNotFound       = newError(1006, "Function argument '%v' was not supplied")
	ErrorFunctionArgUnknown        = newError(1007, "Function argument '%v' was not recognized")
	ErrorIllegalReadonlyLValue     = newError(1008, "A readonly target cannot be used as an assignment target")
)
