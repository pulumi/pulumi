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

import { EngineEvent } from "./events";
import { LocalWorkspace } from "./localWorkspace";
import {
    DestroyResult,
    OutputMap,
    PreviewResult,
    RefreshResult,
    Stack,
    UpdateSummary,
    UpResult,
} from "./stack";
import { Deployment } from "./workspace";

/**
 * RemoteStack is an isolated, independencly configurable instance of a Pulumi program that is
 * operated on remotely (up/preview/refresh/destroy).
 */
export class RemoteStack {
    /** @internal */
    static create(stack: Stack): RemoteStack {
        return new RemoteStack(stack);
    }

    private constructor(private readonly stack: Stack) {
        const ws = stack.workspace;
        if (!(ws instanceof LocalWorkspace)) {
            throw new Error("expected workspace to be an instance of LocalWorkspace");
        }
    }

    /**
     * The name identifying the Stack.
     */
    get name(): string {
        return this.stack.name;
    }

    /**
     * Creates or updates the resources in a stack by executing the program in the Workspace.
     * https://www.pulumi.com/docs/reference/cli/pulumi_up/
     * This operation runs remotely.
     *
     * @param opts Options to customize the behavior of the update.
     */
    up(opts?: RemoteUpOptions): Promise<UpResult> {
        return this.stack.up(opts);
    }

    /**
     * Performs a dry-run update to a stack, returning pending changes.
     * https://www.pulumi.com/docs/reference/cli/pulumi_preview/
     * This operation runs remotely.
     *
     * @param opts Options to customize the behavior of the preview.
     */
    preview(opts?: RemotePreviewOptions): Promise<PreviewResult> {
        return this.stack.preview(opts);
    }

    /**
     * Compares the current stack’s resource state with the state known to exist in the actual
     * cloud provider. Any such changes are adopted into the current stack.
     * This operation runs remotely.
     *
     * @param opts Options to customize the behavior of the refresh.
     */
    refresh(opts?: RemoteRefreshOptions): Promise<RefreshResult> {
        return this.stack.refresh(opts);
    }

    /**
     * Destroy deletes all resources in a stack, leaving all history and configuration intact.
     * This operation runs remotely.
     *
     * @param opts Options to customize the behavior of the destroy.
     */
    destroy(opts?: RemoteDestroyOptions): Promise<DestroyResult> {
        return this.stack.destroy(opts);
    }

    /**
     * Gets the current set of Stack outputs from the last Stack.up().
     */
    outputs(): Promise<OutputMap> {
        return this.stack.outputs();
    }

    /**
     * Returns a list summarizing all previous and current results from Stack lifecycle operations
     * (up/preview/refresh/destroy).
     */
    history(pageSize?: number, page?: number): Promise<UpdateSummary[]> {
        // TODO: Find a way to allow showSecrets as an option that doesn't require loading the project.
        return this.stack.history(pageSize, page, false);
    }

    /**
     * Cancel stops a stack's currently running update. It returns an error if no update is currently running.
     * Note that this operation is _very dangerous_, and may leave the stack in an inconsistent state
     * if a resource operation was pending when the update was canceled.
     * This command is not supported for local backends.
     */
    cancel(): Promise<void> {
        return this.stack.cancel();
    }

    /**
     * exportStack exports the deployment state of the stack.
     * This can be combined with Stack.importStack to edit a stack's state (such as recovery from failed deployments).
     */
    exportStack(): Promise<Deployment> {
        return this.stack.exportStack();
    }

    /**
     * importStack imports the specified deployment state into a pre-existing stack.
     * This can be combined with Stack.exportStack to edit a stack's state (such as recovery from failed deployments).
     *
     * @param state the stack state to import.
     */
    importStack(state: Deployment): Promise<void> {
        return this.stack.importStack(state);
    }
}

/**
 * Options controlling the behavior of a RemoteStack.up() operation.
 */
export interface RemoteUpOptions {
    onOutput?: (out: string) => void;
    onEvent?: (event: EngineEvent) => void;
}

/**
 * Options controlling the behavior of a RemoteStack.preview() operation.
 */
export interface RemotePreviewOptions {
    onOutput?: (out: string) => void;
    onEvent?: (event: EngineEvent) => void;
}

/**
 * Options controlling the behavior of a RemoteStack.refresh() operation.
 */
export interface RemoteRefreshOptions {
    onOutput?: (out: string) => void;
    onEvent?: (event: EngineEvent) => void;
}

/**
 * Options controlling the behavior of a RemoteStack.destroy() operation.
 */
export interface RemoteDestroyOptions {
    onOutput?: (out: string) => void;
    onEvent?: (event: EngineEvent) => void;
}
