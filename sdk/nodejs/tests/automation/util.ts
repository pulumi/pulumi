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

import { randomUUID } from "crypto";
import * as tmp from "tmp";

import { LocalWorkspaceOptions, ProjectRuntime } from "../../automation";

/** @internal */
export function getTestSuffix() {
    return randomUUID();
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

/**
 * Augments the provided {@link LocalWorkspaceOptions} so that they reference a
 * either a file backend or a cloud backend, depending on whether PULUMI_ACCESS_TOKEN
 * is set in the environment.
 *
 * @internal
 */
export function withTestBackend(
    opts?: LocalWorkspaceOptions,
    name?: string,
    description?: string,
    runtime?: string,
): LocalWorkspaceOptions {
    if (process.env.PULUMI_ACCESS_TOKEN) {
        return withCloudBackend(opts, name, description, runtime);
    }
    return withTemporaryFileBackend(opts, name, description, runtime);
}

function withCloudBackend(
    opts?: LocalWorkspaceOptions,
    name?: string,
    description?: string,
    runtime?: string,
): LocalWorkspaceOptions {
    let url = "https://api.pulumi.com";
    if (process.env.PULUMI_BACKEND_URL) {
        url = process.env.PULUMI_BACKEND_URL;
    }
    const backend = {
        url: url,
    };
    if (name === undefined) {
        name = "node_test";
    }
    if (runtime === undefined) {
        runtime = "nodejs";
    }
    return {
        ...opts,
        projectSettings: {
            // We are obliged to provide a name and runtime if we provide project
            // settings, so we do so, but we spread in the provided project settings
            // afterwards so that the caller can override them if need be.
            name: name,
            runtime: runtime as ProjectRuntime,
            description: description,

            ...opts?.projectSettings,
            backend,
        },
    };
}

function withTemporaryFileBackend(
    opts?: LocalWorkspaceOptions,
    name?: string,
    description?: string,
    runtime?: string,
): LocalWorkspaceOptions {
    const tmpDir = tmp.dirSync({
        prefix: "nodejs-tests-automation-",
        unsafeCleanup: true,
    });

    const backend = { url: `file://${tmpDir.name}` };

    if (name === undefined) {
        name = "node_test";
    }
    if (runtime === undefined) {
        runtime = "nodejs";
    }

    return withTestConfigPassphrase({
        ...opts,
        pulumiHome: tmpDir.name,
        projectSettings: {
            // We are obliged to provide a name and runtime if we provide project
            // settings, so we do so, but we spread in the provided project settings
            // afterwards so that the caller can override them if need be.
            name: name,
            runtime: runtime as ProjectRuntime,
            description: description,

            ...opts?.projectSettings,
            backend,
        },
    });
}

/**
 * Augments the provided {@link LocalWorkspaceOptions} so that they set up an
 * environment containing a test `PULUMI_CONFIG_PASSPHRASE` variable suitable
 * for use with a local file backend.
 */
function withTestConfigPassphrase(opts?: LocalWorkspaceOptions): LocalWorkspaceOptions {
    return {
        ...opts,
        envVars: {
            ...opts?.envVars,
            PULUMI_CONFIG_PASSPHRASE: "test",
        },
    };
}
