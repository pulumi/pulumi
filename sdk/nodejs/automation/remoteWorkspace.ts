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

import { LocalWorkspace, LocalWorkspaceOptions } from "./localWorkspace";
import { RemoteStack } from "./remoteStack";
import { Stack } from "./stack";

/**
 * RemoteWorkspace is the execution context containing a single remote Pulumi project.
 */
export class RemoteWorkspace {
    /**
     * PREVIEW: Creates a Stack backed by a RemoteWorkspace with source code from the specified Git repository.
     * Pulumi operations on the stack (Preview, Update, Refresh, and Destroy) are performed remotely.
     *
     * @param args A set of arguments to initialize a RemoteStack with a remote Pulumi program from a Git repository.
     * @param opts Additional customizations to be applied to the Workspace.
     */
    static async createStack(args: RemoteGitProgramArgs, opts?: RemoteWorkspaceOptions): Promise<RemoteStack> {
        const ws = await createLocalWorkspace(args, opts);
        const stack = await Stack.create(args.stackName, ws);
        return RemoteStack.create(stack);
    }

    /**
     * PREVIEW: Selects an existing Stack backed by a RemoteWorkspace with source code from the specified Git
     * repository. Pulumi operations on the stack (Preview, Update, Refresh, and Destroy) are performed remotely.
     *
     * @param args A set of arguments to initialize a RemoteStack with a remote Pulumi program from a Git repository.
     * @param opts Additional customizations to be applied to the Workspace.
     */
    static async selectStack(args: RemoteGitProgramArgs, opts?: RemoteWorkspaceOptions): Promise<RemoteStack> {
        const ws = await createLocalWorkspace(args, opts);
        const stack = await Stack.select(args.stackName, ws);
        return RemoteStack.create(stack);
    }
    /**
     * PREVIEW: Creates or selects an existing Stack backed by a RemoteWorkspace with source code from the specified
     * Git repository. Pulumi operations on the stack (Preview, Update, Refresh, and Destroy) are performed remotely.
     *
     * @param args A set of arguments to initialize a RemoteStack with a remote Pulumi program from a Git repository.
     * @param opts Additional customizations to be applied to the Workspace.
     */
    static async createOrSelectStack(args: RemoteGitProgramArgs, opts?: RemoteWorkspaceOptions): Promise<RemoteStack> {
        const ws = await createLocalWorkspace(args, opts);
        const stack = await Stack.createOrSelect(args.stackName, ws);
        return RemoteStack.create(stack);
    }

    private constructor() {} // eslint-disable-line @typescript-eslint/no-empty-function
}

/**
 * Description of a stack backed by a remote Pulumi program in a Git repository.
 */
export interface RemoteGitProgramArgs {
    /**
     * The name of the associated Stack
     */
    stackName: string;

    /**
     * The URL of the repository.
     */
    url?: string;

    /**
     * Optional path relative to the repo root specifying location of the Pulumi program.
     */
    projectPath?: string;

    /**
     * Optional branch to checkout.
     */
    branch?: string;

    /**
     * Optional commit to checkout.
     */
    commitHash?: string;

    /**
     * Authentication options for the repository.
     */
    auth?: RemoteGitAuthArgs;
}

/**
 * Authentication options for the repository that can be specified for a private Git repo.
 * There are three different authentication paths:
 *  - Personal accesstoken
 *  - SSH private key (and its optional password)
 *  - Basic auth username and password
 *
 * Only one authentication path is valid.
 */
export interface RemoteGitAuthArgs {
    /**
     * The absolute path to a private key for access to the git repo.
     */
    sshPrivateKeyPath?: string;

    /**
     * The (contents) private key for access to the git repo.
     */
    sshPrivateKey?: string;

    /**
     * The password that pairs with a username or as part of an SSH Private Key.
     */
    password?: string;

    /**
     * PersonalAccessToken is a Git personal access token in replacement of your password.
     */
    personalAccessToken?: string;

    /**
     * Username is the username to use when authenticating to a git repository
     */
    username?: string;
}

/**
 * Extensibility options to configure a RemoteWorkspace.
 */
export interface RemoteWorkspaceOptions {
    /**
     * Environment values scoped to the remote workspace. These will be passed to remote operations.
     */
    envVars?: { [key: string]: string | { secret: string } };

    /**
     * An optional list of arbitrary commands to run before a remote Pulumi operation is invoked.
     */
    preRunCommands?: string[];

    /**
     * Whether to skip the default dependency installation step. Defaults to false.
     */
    skipInstallDependencies?: boolean;

    /**
     * Whether to inherit the deployment settings set on the stack. Defaults to false.
     */
    inheritSettings?: boolean;
}

async function createLocalWorkspace(
    args: RemoteGitProgramArgs,
    opts?: RemoteWorkspaceOptions,
): Promise<LocalWorkspace> {
    if (!isFullyQualifiedStackName(args.stackName)) {
        throw new Error(`stack name "${args.stackName}" must be fully qualified.`);
    }

    if (!args.url && !opts?.inheritSettings) {
        throw new Error("url is required if inheritSettings is not set.");
    }
    if (args.branch && args.commitHash) {
        throw new Error("branch and commitHash cannot both be specified.");
    }
    if (!args.branch && !args.commitHash && !opts?.inheritSettings) {
        throw new Error("either branch or commitHash is required if inheritSettings is not set.");
    }
    if (args.auth) {
        if (args.auth.sshPrivateKey && args.auth.sshPrivateKeyPath) {
            throw new Error("sshPrivateKey and sshPrivateKeyPath cannot both be specified.");
        }
    }

    const localOpts: LocalWorkspaceOptions = {
        remote: true,
        remoteGitProgramArgs: args,
        remoteEnvVars: opts?.envVars,
        remotePreRunCommands: opts?.preRunCommands,
        remoteSkipInstallDependencies: opts?.skipInstallDependencies,
        remoteInheritSettings: opts?.inheritSettings,
    };
    return await LocalWorkspace.create(localOpts);
}

/** @internal exported only so it can be tested */
export function isFullyQualifiedStackName(stackName: string): boolean {
    if (!stackName) {
        return false;
    }
    const split = stackName.split("/");
    return split.length === 3 && !!split[0] && !!split[1] && !!split[2];
}
