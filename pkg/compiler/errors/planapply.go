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

package errors

// Plan and apply errors are in the [2000,3000) range.
var (
	ErrorCantCreateCompiler     = newError(2000, "An error occurred during compiler construction: %v")
	ErrorCantReadPackage        = newError(2001, "An error occurred while reading the package '%v': %v")
	ErrorCantCreateSnapshot     = newError(2002, "A problem was found during planning: %v")
	ErrorPlanApplyFailed        = newError(2003, "Plan apply failed: %v")
	ErrorIllegalMarkupExtension = newError(2004, "Resource serialization failed; illegal markup extension '%v'")
	ErrorCantReadDeployment     = newError(2005, "Could not read deployment file '%v': %v")
	ErrorDuplicateURNNames      = newError(2006, "Duplicate objects with the same URN: %v")
	ErrorInvalidEnvName         = newError(2007, "Environment '%v' could not be found in the current workspace")
	ErrorIllegalConfigToken     = newError(2008,
		"Configs may only target module properties and class static properties; '%v' is neither")
	ErrorConfigApplyFailure           = newError(2009, "One or more errors occurred while applying '%v's configuration")
	ErrorResourcePropertyInvalidValue = newError(2010, "Resource '%v's property '%v' value %v has a problem: %v")
	ErrorAnalyzeFailure               = newError(2011, "Analyzer '%v' reported an error: %v")
	ErrorAnalyzeResourceFailure       = newError(2012,
		"Analyzer '%v' reported a resource error:\n"+
			"\tResource: %v\n"+
			"\tProperty: %v\n"+
			"\tReason: %v")
)
