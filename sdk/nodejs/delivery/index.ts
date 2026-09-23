// Copyright 2026, Pulumi Corporation.
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

import { all, Input, Output, output } from "../output";
import { CustomResource, CustomResourceOptions } from "../resource";

/**
 * Where a delivered stack's program is.
 */
export interface StackSource {
    /**
     * The directory holding the program, relative to the delivery program's own.
     */
    readonly directory: Input<string>;
}

export interface StackArgs {
    /**
     * The fully qualified name of the stack to deliver, as `<organization>/<project>/<stack>`.
     */
    readonly stack: Input<string>;
    readonly source: Input<StackSource>;
}

/**
 * Delivers a Pulumi stack: previewing the delivery program previews the stack, and updating it
 * updates the stack, in the same coherence window as every other stack the delivery program
 * delivers. The stack's outputs are available through {@link Stack.outputs} or
 * {@link Stack.getOutput}.
 */
export class Stack extends CustomResource {
    public readonly stack!: Output<string>;
    public readonly outputs!: Output<{ [name: string]: any }>;
    public readonly secretOutputNames!: Output<string[]>;

    constructor(name: string, args: StackArgs, opts?: CustomResourceOptions) {
        super(
            "pulumi:delivery:Stack",
            name,
            {
                stack: args.stack,
                source: args.source,
                outputs: undefined,
                secretOutputNames: undefined,
            },
            opts,
        );
    }

    /**
     * Fetches the value of the named stack output, or `undefined` if the stack output was not found.
     */
    public getOutput(name: Input<string>): Output<any> {
        return outputNamed(this.outputs, this.secretOutputNames, name, false);
    }

    /**
     * Fetches the value of the named stack output, or throws an error if the output was not found.
     */
    public requireOutput(name: Input<string>): Output<any> {
        return outputNamed(this.outputs, this.secretOutputNames, name, true);
    }
}

export interface StackGroupArgs {
    /**
     * The stacks to deliver together. They start at once, and a stack that reads another's outputs
     * waits for that stack's own preview or update rather than reading its saved state.
     */
    readonly stacks: Input<Input<StackArgs>[]>;
}

export interface StackGroupMember {
    readonly stack: string;
    readonly outputs: { [name: string]: any };
    readonly secretOutputNames: string[];
}

/**
 * Delivers a set of Pulumi stacks together without knowing the order they depend on each other in.
 */
export class StackGroup extends CustomResource {
    public readonly members!: Output<StackGroupMember[]>;

    constructor(name: string, args: StackGroupArgs, opts?: CustomResourceOptions) {
        super(
            "pulumi:delivery:StackGroup",
            name,
            {
                stacks: args.stacks,
                members: undefined,
            },
            opts,
        );
    }

    /**
     * Fetches the outputs of one of the group's stacks.
     */
    public stackOutputs(stack: Input<string>): Output<{ [name: string]: any }> {
        return all([output(stack), this.members]).apply(([s, members]) => member(members, s).outputs);
    }

    /**
     * Fetches the value of the named output of one of the group's stacks, or `undefined` if the
     * stack output was not found.
     */
    public getOutput(stack: Input<string>, name: Input<string>): Output<any> {
        return outputNamed(this.stackOutputs(stack), this.secretOutputNames(stack), name, false);
    }

    /**
     * Fetches the value of the named output of one of the group's stacks, or throws an error if the
     * output was not found.
     */
    public requireOutput(stack: Input<string>, name: Input<string>): Output<any> {
        return outputNamed(this.stackOutputs(stack), this.secretOutputNames(stack), name, true);
    }

    private secretOutputNames(stack: Input<string>): Output<string[]> {
        return all([output(stack), this.members]).apply(([s, members]) => member(members, s).secretOutputNames);
    }
}

function member(members: StackGroupMember[], stack: string): StackGroupMember {
    const found = members.find((m) => m.stack === stack);
    if (found === undefined) {
        throw new Error(`Stack '${stack}' is not a member of this stack group.`);
    }
    return found;
}

function outputNamed(
    outputs: Output<{ [name: string]: any }>,
    secretOutputNames: Output<string[]>,
    name: Input<string>,
    required: boolean,
): Output<any> {
    // A plain `apply` marks its result secret whenever `outputs` holds any secret; deciding from the
    // names keeps outputs that are not secret from being tainted.
    const value = all([output(name), outputs]).apply(([n, os]) => {
        if (required && !os.hasOwnProperty(n)) {
            throw new Error(`Required output '${n}' does not exist.`);
        }
        return os[n];
    });
    return new Output(
        value.resources(),
        value.promise(),
        value.isKnown,
        isSecretOutputName(outputs, secretOutputNames, name),
        value.allResources!(),
    );
}

async function isSecretOutputName(
    outputs: Output<{ [name: string]: any }>,
    secretOutputNames: Output<string[]>,
    name: Input<string>,
): Promise<boolean> {
    const nameOutput = output(name);
    if (!((await nameOutput.isKnown) && (await secretOutputNames.isKnown))) {
        return await outputs.isSecret;
    }
    const names = await secretOutputNames.promise();
    if (!names) {
        return await outputs.isSecret;
    }
    return names.includes(await nameOutput.promise());
}
