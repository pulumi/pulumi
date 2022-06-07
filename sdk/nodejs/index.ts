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

// Enable source map support so we get good stack traces.
import "source-map-support/register";

// Export top-level elements.
export * from "./config";
export * from "./errors";
export * from "./invoke";
export * from "./metadata";
export * from "./output";
export * from "./resource";
export * from "./stackReference";

// Export submodules individually.
import * as asset from "./asset";
import * as automation from "./automation";
import * as dynamic from "./dynamic";
import * as iterable from "./iterable";
import * as log from "./log";
import * as provider from "./provider";
import * as runtime from "./runtime";
import * as utils from "./utils";
export { asset, automation, dynamic, iterable, log, provider, runtime, utils };

// @pulumi is a deployment-only module.  If someone tries to capture it, and we fail for some reason
// we want to give a good message about what the problem likely is.  Note that capturing a
// deployment time module can be ok in some cases.  For example, using "new pulumi.Config" is fine.
// However, in general, the majority of this API is not safe to use at 'run time' and will fail.
/** @internal */
export const deploymentOnlyModule = true;
