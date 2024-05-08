// Copyright 2024-2024, Pulumi Corporation.
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

import * as deploymentOnlyModule from "./deploymentOnlyModule";

export const description = "Capture config created on the outside";

// Used just to validate that if we capture a Config object we see these values serialized over.
// Specifically, the module that Config uses needs to be captured by value and not be
// 'require-reference'.
deploymentOnlyModule.setConfig("test:TestingKey1", "TestingValue1");
const testConfig = new deploymentOnlyModule.Config("test");

export const func = function () { const v = testConfig.get("TestingKey1"); console.log(v); };
