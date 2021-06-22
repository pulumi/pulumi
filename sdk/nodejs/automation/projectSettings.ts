
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

/**
 * A Pulumi project manifest. It describes metadata applying to all sub-stacks created from the project.
 */
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

/**
 * A description of the Project's program runtime and associated metadata.
 */
export interface ProjectRuntimeInfo {
    name: string;
    options?: { [key: string]: any };
}

/**
 * Supported Pulumi program language runtimes.
 */
export type ProjectRuntime = "nodejs" | "go" | "python" | "dotnet";

/**
 * A template used to seed new stacks created from this project.
 */
export interface ProjectTemplate {
    description?: string;
    quickstart?: string;
    config?: { [key: string]: ProjectTemplateConfigValue };
    important?: boolean;
}

/**
 * A placeholder config value for a project template.
 */
export interface ProjectTemplateConfigValue {
    description?: string;
    default?: string;
    secret?: boolean;
}

/**
 * Configuration for the project's Pulumi state storage backend.
 */
export interface ProjectBackend {
    url?: string;
}
