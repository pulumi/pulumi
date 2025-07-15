// Copyright 2016-2022, Pulumi Corporation.
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

import { v4 as uuidv4 } from "uuid";

/** @internal */
export function getTestSuffix() {
    return uuidv4();
}

/** @internal */
export function getTestOrg() {
    if (process.env.PULUMI_TEST_ORG) {
        return process.env.PULUMI_TEST_ORG;
    }
    if (process.env.PULUMI_ACCESS_TOKEN) {
        return "pulumi-test";
    }
    return "organization";
}
