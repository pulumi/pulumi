
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

export interface ProjectSettings {
    name: string;
    runtime: ProjectRuntimeInfo | ProjectRuntime;
    main?: string;
    description?: string;
    author?: string;
    website?: string;
    license?: string;
    config?: string;
    template?: ProjectTemplate;
    backend?: ProjectBackend;
}

export interface ProjectRuntimeInfo {
    name: string;
    options?: { [key: string]: any };
}

export type ProjectRuntime = "nodejs" | "go" | "python" | "dotnet";

export interface ProjectTemplate {
    description?: string;
    quickstart?: string;
    config?: { [key: string]: ProjectTemplateConfigValue };
    important?: boolean;
}

export interface ProjectTemplateConfigValue {
    description?: string;
    default?: string;
    secret?: boolean;
}

export interface ProjectBackend {
    url?: string;
}
