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

import { ConfigMap, ConfigValue } from "./config";
import { ProjectSettings } from "./projectSettings";
import { StackSettings } from "./stackSettings";

export interface Workspace {
    readonly workDir: string;
    readonly pulumiHome?: string;
    readonly secretsProvider?: string;
    program?: PulumiFn;
    envVars: { [key: string]: string };
    projectSettings(): Promise<ProjectSettings>;
    saveProjectSettings(settings: ProjectSettings): Promise<void>;
    stackSettings(stackName: string): Promise<StackSettings>;
    saveStackSettings(settings: StackSettings, stackName: string): Promise<void>;
    serializeArgsForOp(stackName: string): Promise<string[]>;
    postCommandCallback(stackName: string): Promise<void>;
    getConfig(stackName: string, key: string): Promise<ConfigValue>;
    getAllConfig(stackName: string): Promise<ConfigMap>;
    setConfig(stackName: string, key: string, value: ConfigValue): Promise<void>;
    setAllConfig(stackName: string, config: ConfigMap): Promise<void>;
    removeConfig(stackName: string, key: string): Promise<void>;
    removeAllConfig(stackName: string, keys: string[]): Promise<void>;
    refreshConfig(stackName: string): Promise<ConfigMap>;
    whoAmI(): Promise<WhoAmIResult>;
    stack(): Promise<StackSummary | undefined>;
    createStack(stackName: string): Promise<void>;
    selectStack(stackName: string): Promise<void>;
    removeStack(stackName: string): Promise<void>;
    listStacks(): Promise<StackSummary[]>;
    // TODO import/export
}

export type StackSummary = {
    name: string,
    current: boolean,
    lastUpdate?: string,
    updateInProgress: boolean,
    resourceCount?: number,
    url?: string,
};

export type PulumiFn = () => Promise<Record<string, any> | void>;

export interface WhoAmIResult {
    user: string;
}
