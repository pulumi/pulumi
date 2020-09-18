
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

import * as yaml from "js-yaml";

export class ProjectSettings {
    name: string;
    runtime: ProjectRuntimeInfo;
    main?: string;
    description?: string;
    author?: string;
    website?: string;
    license?: string;
    config?: string;
    template?: ProjectTemplate;
    backend?: ProjectBackend;

    public static fromJSON(obj: any) {
        const proj = new ProjectSettings();

        if (!obj.runtime || !obj.name) {
            throw new Error("could not deserialize ProjectSettings, missing required properties");
        }

        proj.name = obj.name;
        proj.runtime = ProjectRuntimeInfo.fromJSON(obj.runtime);
        proj.main = obj.main;
        proj.description = obj.description;
        proj.author = obj.author;
        proj.website = obj.website;
        proj.license = obj.license;
        proj.config = obj.config;
        proj.template = obj.template;
        proj.backend = obj.backend;

        return proj;
    }

    public static fromYAML(text: string) {
        const res = yaml.safeLoad(text, { json: true });
        return ProjectSettings.fromJSON(res);
    }

    constructor() {
        this.name = "";
        this.runtime = new ProjectRuntimeInfo();
    }

    toYAML(): string {
        const copy = Object.assign({}, this);
        copy.runtime = copy.runtime.toJSON();
        return yaml.safeDump(copy, { skipInvalid: true });
    }

}

export class ProjectRuntimeInfo {
    name: string;
    options?: {[key: string]: any};

    public static fromJSON(obj: any) {
        const info = new ProjectRuntimeInfo();

        if (typeof obj === "string") {
            info.name = obj;
        }
        else {
            info.name = obj.name;
            info.options = obj.options;
        }

        if (!info.name) {
            throw new Error("could not deserialize invalid ProjectRuntimeInfo object");
        }

        return info;
    }

    constructor() {
        this.name = "";
    }

    toJSON(): any {
        if (!this.options) {
            return this.name;
        }

        return this;
    }
}

export type ProjectTemplate = {
    description?: string,
    quickstart?: string,
    config?: {[key: string]: ProjectTemplateConfigValue},
    important?: boolean,
};

export type ProjectTemplateConfigValue = {
    description?: string,
    default?: string,
    secret?: boolean,
};

export type ProjectBackend = {
    url?: string,
};
