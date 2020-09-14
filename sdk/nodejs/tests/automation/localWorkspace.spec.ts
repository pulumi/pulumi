// Copyright 2016-2020, Pulumi Corporation.
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

import * as assert from "assert";
import { ProjectSettings } from "../../x/automation/projectSettings";
import { LocalWorkspace } from "../../x/automation/localWorkspace";

import { asyncTest } from "../util";
import { fileURLToPath } from "url";
import * as upath from "upath";


describe("LocalWorkspace", () => {
    it(`projectSettings from yaml/yml/json`, asyncTest(async () => {
        for (let ext of ["yaml", "yml", "json"]) {
            const ws = new LocalWorkspace({ workDir: upath.joinSafe(__dirname, "data", ext) })
            const settings = await ws.projectSettings();
            assert(settings.name, "testproj")
            assert(settings.runtime.name, "go")
            assert(settings.description, "A minimal Go Pulumi program")

        }
    }))

}) 