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

export const description = "Capture config created on the inside";

deploymentOnlyModule.setConfig("test:TestingKey2", "TestingValue2");

export const func = function () { const v = new deploymentOnlyModule.Config("test").get("TestingKey2"); console.log(v); };
