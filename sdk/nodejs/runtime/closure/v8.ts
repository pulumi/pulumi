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

// This file provides a low-level interface to a few V8 runtime objects. We will use this low-level
// interface when serializing closures to walk the scope chain and find the value of free variables
// captured by closures, as well as getting source-level debug information so that we can present
// high-quality error messages.
//
// As a side-effect of importing this file, we must enable the --allow-natives-syntax V8 flag. This
// is because we are using V8 intrinsics in order to implement this module.
import * as semver from "semver";
import * as v8 from "v8";
v8.setFlagsFromString("--allow-natives-syntax");

import * as v8Hooks from "./v8Hooks";

import * as v8_v10andLower from "./v8_v10andLower";
import * as v8_v11andHigher from "./v8_v11andHigher";

// Node majorly changed their introspection apis between 10.0 and 11.0 (removing outright some
// of the APIs we use).  Detect if we're before or after this change and delegate to the
const versionSpecificV8Module =  v8Hooks.isNodeAtLeastV11 ? v8_v11andHigher : v8_v10andLower;

/**
 * Given a function and a free variable name, lookupCapturedVariableValue looks up the value of that free variable
 * in the scope chain of the provided function. If the free variable is not found, `throwOnFailure` indicates
 * whether or not this function should throw or return `undefined.
 *
 * @param func The function whose scope chain is to be analyzed
 * @param freeVariable The name of the free variable to inspect
 * @param throwOnFailure If true, throws if the free variable can't be found.
 * @returns The value of the free variable. If `throwOnFailure` is false, returns `undefined` if not found.
 */
export const lookupCapturedVariableValueAsync = versionSpecificV8Module.lookupCapturedVariableValueAsync;

/**
 * Given a function, returns the file, line and column number in the file where this function was
 * defined. Returns { "", 0, 0 } if the location cannot be found or if the given function has no Script.
 */
export const getFunctionLocationAsync = versionSpecificV8Module.getFunctionLocationAsync;
