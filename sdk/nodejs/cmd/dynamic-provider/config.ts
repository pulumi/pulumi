// Copyright 2016-2024, Pulumi Corporation.
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

import * as dynamic from "../../dynamic";

const configSeparator = `:`;

export class Config implements dynamic.Config {
    private rawConfig: Record<string, any>;
    private projectName: string;

    constructor(rawConfig: Record<string, any>, projectName: string) {
        this.rawConfig = rawConfig;
        this.projectName = projectName;
    }

    get(key: string): string | undefined {
        if (!key.includes(configSeparator)) {
            key = `${this.projectName}:${key}`;
        }
        return this.rawConfig[key];
    }

    require(key: string): string {
        const value = this.get(key);
        if (value === undefined) {
            throw new Error(`Missing required configuration key: ${key}`);
        }
        return value;
    }
}
