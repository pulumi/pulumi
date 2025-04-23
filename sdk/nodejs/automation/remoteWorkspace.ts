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
 * {@link RemoteWorkspace} is the execution context containing a single remote
 * Pulumi project.
 */
export class RemoteWorkspace {
    /**
     * Creates a stack backed by a {@link RemoteWorkspace} with source code from
     * the specified Git repository. Pulumi operations on the stack (preview,
     * update, refresh, and destroy) are performed remotely.
     *
     * @param args
     *  A set of arguments to initialize a {@link RemoteStack} with a remote
     *  Pulumi program from a Git repository.
     * @param opts
     *  Additional customizations to be applied to the Workspace.
     */
    static async createStack(args: RemoteGitProgramArgs, opts?: RemoteWorkspaceOptions): Promise<RemoteStack> {
        const ws = await createLocalWorkspace(args, opts);
        const stack = await Stack.create(args.stackName, ws);
        return RemoteStack.create(stack);
    }

    /**
     * Selects an existing stack backed by a {@link RemoteWorkspace} with source
     * code from the specified Git repository. Pulumi operations on the stack
     * (preview, update, refresh, and destroy) are performed remotely.
     *
     * @param args
     *  A set of arguments to initialize a {@link RemoteStack} with a remote
     *  Pulumi program from a Git repository.
     * @param opts
     *  Additional customizations to be applied to the Workspace.
     */
    static async selectStack(args: RemoteGitProgramArgs, opts?: RemoteWorkspaceOptions): Promise<RemoteStack> {
        const ws = await createLocalWorkspace(args, opts);
        const stack = await Stack.select(args.stackName, ws);
        return RemoteStack.create(stack);
    }
    /**
     * Creates or selects an existing stack backed by a {@link RemoteWorkspace}
     * with source code from the specified Git repository. Pulumi operations on
     * the stack (preview, update, refresh, and destroy) are performed remotely.
     *
     * @param args
     *  A set of arguments to initialize a RemoteStack with a remote Pulumi program from a Git repository.
     * @param opts
     *  Additional customizations to be applied to the Workspace.
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
     * The associated stack name.
     */
    stackName: string;

    /**
     * The URL of the repository.
     */
    url?: string;

    /**
     * An optional path relative to the repo root specifying location of the Pulumi program.
     */
    projectPath?: string;

    /**
     * An optional branch to checkout.
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
 * Authentication options that can be specified for a private Git repository.
 * There are three different authentication paths:
 *
 *  - A Personal access token
 *  - An SSH private key (and its optional passphrase)
 *  - Username and password (basic authentication)
 *
 * Only one authentication path is valid.
 */
export interface RemoteGitAuthArgs {
    /**
     * The absolute path to a private key to be used for access to the Git repository.
     */
    sshPrivateKeyPath?: string;

    /**
     * A string containing the contents of a private key to be used for access
     * to the Git repository.
     */
    sshPrivateKey?: string;

    /**
     * The password that pairs with a username as part of basic authentication,
     * or the passphrase to be used with an SSH private key.
     */
    password?: string;

    /**
     * A Git personal access token, to be used in replacement of a password.
     */
    personalAccessToken?: string;

    /**
     * The username to use when authenticating to a Git repository with basic
     * authentication.
     */
    username?: string;
}

/**
 * Extensibility options to configure a {@link RemoteWorkspace.}
 */
export interface RemoteWorkspaceOptions {
    /**
     * Environment values scoped to the remote workspace. These will be passed
     * to remote operations.
     */
    envVars?: { [key: string]: string | { secret: string } };

    /**
     * An optional list of arbitrary commands to run before a remote Pulumi
     * operation is invoked.
     */
    preRunCommands?: string[];

    /**
     * Whether to skip the default dependency installation step. Defaults to
     * false.
     */
    skipInstallDependencies?: boolean;

    /**
     * Whether to inherit the deployment settings set on the stack. Defaults to
     * false.
     */
    inheritSettings?: boolean;

    /**
     * The image to use for the remote executor.
     */
    executorImage?: ExecutorImage;
}

/**
 * Information about the remote execution image.
 */
export interface ExecutorImage {
  image: string;
  credentials?: DockerImageCredentials;
}

/**
 * Credentials for the remote execution Docker image.
 */
export interface DockerImageCredentials {
  username: string;
  password: string;
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
        remoteExecutorImage: opts?.executorImage,
    };
    return await LocalWorkspace.create(localOpts);
}

/**
 * @internal
 *  Exported only so it can be tested.
 */
export function isFullyQualifiedStackName(stackName: string): boolean {
    if (!stackName) {
        return false;
    }
    const split = stackName.split("/");
    return split.length === 3 && !!split[0] && !!split[1] && !!split[2];
}
