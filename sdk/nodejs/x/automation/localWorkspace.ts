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

import * as fs from "fs";
import * as upath from "upath";
import { CommandResult, runPulumiCmd } from "./cmd";
import { ProjectSettings } from "./projectSettings";
import { StackSettings } from "./stackSettings";
import { Workspace } from "./workspace";


export class LocalWorkspace /*implements Workspace TODO */ {
    private workDir: string;
    private pulumiHome?: string;
    private program?: () => void;
    private envVars?: { [key: string]: string };
    constructor(opts?: LocalWorkspaceOpts) {
        let dir = "";
        let envs = {};
        if (opts) {
            const {workDir, pulumiHome, program, envVars} = opts;
            if (workDir) {
                dir = workDir;
            }
            this.pulumiHome = pulumiHome;
            this.program = program;
            envs = {...envVars};
        }

        if (!dir) {
            dir = fs.mkdtempSync("automation-");
        }
        this.workDir = dir;
        this.envVars = envs;
    }
    projectSettings(): Promise<ProjectSettings> {
        for (const ext of settingsExtensions) {
            const isJSON = ext === ".json";
            const path = upath.joinSafe(this.workDir, `Pulumi${ext}`);
            if (!fs.existsSync(path)) { continue; }
            const contents = fs.readFileSync(path).toString();
            if (isJSON) {
                return Promise.resolve(ProjectSettings.fromJSON(JSON.parse(contents)));
            }
            return Promise.resolve(ProjectSettings.fromYAML(contents));
        }
        return Promise.reject(new Error(`failed to find project settings file in workdir: ${this.workDir}`));
    }
    saveProjectSettings(settings: ProjectSettings): Promise<void> {
        let foundExt = ".yaml";
        for (const ext of settingsExtensions) {
           const testPath = upath.joinSafe(this.workDir, `Pulumi${ext}`);
           if (fs.existsSync(testPath)) {
               foundExt = ext;
               break;
           }
        }
        const path = upath.joinSafe(this.workDir, `Pulumi${foundExt}`);
        let contents;
        if (foundExt === ".json") {
            contents = JSON.stringify(settings, null, 4);
        }
        else {
            contents = settings.toYAML();
        }
        return Promise.resolve(fs.writeFileSync(path, contents));
    }
    stackSettings(stackName: string): Promise<StackSettings> {
        const stackSettingsName = getStackSettingsName(stackName);
        for (const ext of settingsExtensions) {
            const isJSON = ext === ".json";
            const path = upath.joinSafe(this.workDir, `Pulumi.${stackSettingsName}${ext}`);
            if (!fs.existsSync(path)) { continue; }
            const contents = fs.readFileSync(path).toString();
            if (isJSON) {
                return Promise.resolve(StackSettings.fromJSON(JSON.parse(contents)));
            }
            return Promise.resolve(StackSettings.fromYAML(contents));
        }
        return Promise.reject(new Error(`failed to find stack settings file in workdir: ${this.workDir}`));
    }
    saveStackSettings(settings: StackSettings, stackName: string): Promise<void> {
        const stackSettingsName = getStackSettingsName(stackName);
        let foundExt = ".yaml";
        for (const ext of settingsExtensions) {
           const testPath = upath.joinSafe(this.workDir, `Pulumi.${stackSettingsName}${ext}`);
           if (fs.existsSync(testPath)) {
               foundExt = ext;
               break;
           }
        }
        const path = upath.joinSafe(this.workDir, `Pulumi.${stackSettingsName}${foundExt}`);
        let contents;
        if (foundExt === ".json") {
            contents = JSON.stringify(settings, null, 4);
        }
        else {
            contents = settings.toYAML();
        }
        return Promise.resolve(fs.writeFileSync(path, contents));
    }
    serializeArgsForOp(_: string): string[] {
        // LocalWorkspace does not take advantage of this extensibility point.
        return [];
    }
    postCommandCallback(_: string): void {
        // LocalWorkspace does not take advantage of this extensibility point.
        return;
    }
    runPulumiCmd(
        args: string[],
        onOutput?: (data: string) => void,
    ): Promise<CommandResult> {
        return runPulumiCmd(args, this.workDir, {});
    }
}

export type LocalWorkspaceOpts = {
    workDir?: string,
    pulumiHome?: string,
    program?: ()=>void,
    envVars?: { [key: string]: string },
};

export const settingsExtensions = [".yaml", ".yml", ".json"];

const getStackSettingsName = (name: string): string => {
    const parts = name.split("/");
    if (parts.length < 1) {
        return name;
    }
    return parts[parts.length - 1];
};
