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

import * as fs from "fs";
import * as typescript from "typescript";

import * as log from "./log";

export function loadTypeScriptCompilerOptions(tsConfigPath: string): object {
    try {
        const tsConfigString = fs.readFileSync(tsConfigPath).toString();
        // Using local `require("typescript")` to avoid always loading
        // and only load on-demand, avoid up to 300s overhead in Node runtime.
        const ts: typeof typescript = require("typescript");
        const tsConfig = ts.parseConfigFileTextToJson(tsConfigPath, tsConfigString).config;
        return tsConfig["compilerOptions"] ?? {};
    } catch (err) {
        log.debug(`Ignoring error in loadCompilerOptions(${tsConfigPath}}): ${err}`);
        return {};
    }
}
