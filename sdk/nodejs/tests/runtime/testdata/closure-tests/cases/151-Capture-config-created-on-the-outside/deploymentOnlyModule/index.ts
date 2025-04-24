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

// This module is use for testing to ensure that capturing a value from inside a deployment-time
// module works properly.  We replicate the 'Config' type here as that's the principle case
// of a value we want people to be able to capture from a project like pulumi.

export * from "./config";
export * from "./runtimeConfig";

// simulate this being a deployment-time only module.
export const deploymentOnlyModule = true;
