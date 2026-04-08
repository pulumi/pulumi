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

import * as process from "process";
import type { CommandResult } from "../cmd";
import { PulumiCommand } from "../cmd";

export type BaseOptions = {
    cwd?: string;
    additionalEnv?: { [key: string]: string };
    onOutput?: (data: string) => void;
    onError?: (data: string) => void;
    signal?: AbortSignal;
};

export class API {
    private _command: PulumiCommand;

    constructor(command: PulumiCommand) {
        this._command = command;
    }

    private __run(options: BaseOptions, args: string[]): Promise<CommandResult> {
        return this._command.run(
            args,
            options.cwd ?? process.cwd(),
            options.additionalEnv ?? {},
            options.onOutput,
            options.onError,
            options.signal,
        );
    }

    cancel(options: PulumiCancelOptions, stackName?: string): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("cancel");

        const __flags: string[] = [];

        __flags.push("--yes");

        if (options.color != null) {
            __flags.push("--color", "" + options.color);
        }

        if (options.disableIntegrityChecking) {
            __flags.push("--disable-integrity-checking");
        }

        if (options.fullyQualifyStackNames) {
            __flags.push("--fully-qualify-stack-names");
        }

        if (options.logflow) {
            __flags.push("--logflow");
        }

        if (options.logtostderr) {
            __flags.push("--logtostderr");
        }

        if (options.memprofilerate != null) {
            __flags.push("--memprofilerate", "" + options.memprofilerate);
        }

        if (options.otelTraces != null) {
            __flags.push("--otel-traces", "" + options.otelTraces);
        }

        if (options.profiling != null) {
            __flags.push("--profiling", "" + options.profiling);
        }

        if (options.tracing != null) {
            __flags.push("--tracing", "" + options.tracing);
        }

        if (options.tracingHeader != null) {
            __flags.push("--tracing-header", "" + options.tracingHeader);
        }

        if (options.verbose != null) {
            __flags.push("--verbose", "" + options.verbose);
        }

        if (options.stack != null) {
            __flags.push("--stack", "" + options.stack);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (stackName != null) {
            __arguments.push("" + stackName);
        }
        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    new(options: PulumiNewOptions, templateOrUrl?: string): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("new");

        const __flags: string[] = [];

        __flags.push("--yes");

        if (options.color != null) {
            __flags.push("--color", "" + options.color);
        }

        if (options.disableIntegrityChecking) {
            __flags.push("--disable-integrity-checking");
        }

        if (options.fullyQualifyStackNames) {
            __flags.push("--fully-qualify-stack-names");
        }

        if (options.logflow) {
            __flags.push("--logflow");
        }

        if (options.logtostderr) {
            __flags.push("--logtostderr");
        }

        if (options.memprofilerate != null) {
            __flags.push("--memprofilerate", "" + options.memprofilerate);
        }

        if (options.otelTraces != null) {
            __flags.push("--otel-traces", "" + options.otelTraces);
        }

        if (options.profiling != null) {
            __flags.push("--profiling", "" + options.profiling);
        }

        if (options.tracing != null) {
            __flags.push("--tracing", "" + options.tracing);
        }

        if (options.tracingHeader != null) {
            __flags.push("--tracing-header", "" + options.tracingHeader);
        }

        if (options.verbose != null) {
            __flags.push("--verbose", "" + options.verbose);
        }

        if (options.ai != null) {
            __flags.push("--ai", "" + options.ai);
        }

        for (const __item of options.config ?? []) {
            if (__item != null) {
                __flags.push("--config", "" + __item);
            }
        }

        if (options.configPath) {
            __flags.push("--config-path");
        }

        if (options.description != null) {
            __flags.push("--description", "" + options.description);
        }

        if (options.dir != null) {
            __flags.push("--dir", "" + options.dir);
        }

        if (options.force) {
            __flags.push("--force");
        }

        if (options.generateOnly) {
            __flags.push("--generate-only");
        }

        if (options.language != null) {
            __flags.push("--language", "" + options.language);
        }

        if (options.listTemplates) {
            __flags.push("--list-templates");
        }

        if (options.name != null) {
            __flags.push("--name", "" + options.name);
        }

        if (options.offline) {
            __flags.push("--offline");
        }

        if (options.remoteStackConfig) {
            __flags.push("--remote-stack-config");
        }

        for (const __item of options.runtimeOptions ?? []) {
            if (__item != null) {
                __flags.push("--runtime-options", "" + __item);
            }
        }

        if (options.secretsProvider != null) {
            __flags.push("--secrets-provider", "" + options.secretsProvider);
        }

        if (options.stack != null) {
            __flags.push("--stack", "" + options.stack);
        }

        if (options.templateMode) {
            __flags.push("--template-mode");
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (templateOrUrl != null) {
            __arguments.push("" + templateOrUrl);
        }
        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    orgGetDefault(options: PulumiOrgGetDefaultOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("org");
        __final.push("get-default");

        const __flags: string[] = [];

        if (options.color != null) {
            __flags.push("--color", "" + options.color);
        }

        if (options.disableIntegrityChecking) {
            __flags.push("--disable-integrity-checking");
        }

        if (options.fullyQualifyStackNames) {
            __flags.push("--fully-qualify-stack-names");
        }

        if (options.logflow) {
            __flags.push("--logflow");
        }

        if (options.logtostderr) {
            __flags.push("--logtostderr");
        }

        if (options.memprofilerate != null) {
            __flags.push("--memprofilerate", "" + options.memprofilerate);
        }

        if (options.otelTraces != null) {
            __flags.push("--otel-traces", "" + options.otelTraces);
        }

        if (options.profiling != null) {
            __flags.push("--profiling", "" + options.profiling);
        }

        if (options.tracing != null) {
            __flags.push("--tracing", "" + options.tracing);
        }

        if (options.tracingHeader != null) {
            __flags.push("--tracing-header", "" + options.tracingHeader);
        }

        if (options.verbose != null) {
            __flags.push("--verbose", "" + options.verbose);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    orgSearchAi(options: PulumiOrgSearchAiOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("org");
        __final.push("search");
        __final.push("ai");

        const __flags: string[] = [];

        if (options.color != null) {
            __flags.push("--color", "" + options.color);
        }

        if (options.disableIntegrityChecking) {
            __flags.push("--disable-integrity-checking");
        }

        if (options.fullyQualifyStackNames) {
            __flags.push("--fully-qualify-stack-names");
        }

        if (options.logflow) {
            __flags.push("--logflow");
        }

        if (options.logtostderr) {
            __flags.push("--logtostderr");
        }

        if (options.memprofilerate != null) {
            __flags.push("--memprofilerate", "" + options.memprofilerate);
        }

        if (options.otelTraces != null) {
            __flags.push("--otel-traces", "" + options.otelTraces);
        }

        if (options.profiling != null) {
            __flags.push("--profiling", "" + options.profiling);
        }

        if (options.tracing != null) {
            __flags.push("--tracing", "" + options.tracing);
        }

        if (options.tracingHeader != null) {
            __flags.push("--tracing-header", "" + options.tracingHeader);
        }

        if (options.verbose != null) {
            __flags.push("--verbose", "" + options.verbose);
        }

        if (options.delimiter != null) {
            __flags.push("--delimiter", "" + options.delimiter);
        }

        if (options.org != null) {
            __flags.push("--org", "" + options.org);
        }

        if (options.output != null) {
            __flags.push("--output", "" + options.output);
        }

        if (options.query != null) {
            __flags.push("--query", "" + options.query);
        }

        if (options.web) {
            __flags.push("--web");
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    orgSearch(options: PulumiOrgSearchOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("org");
        __final.push("search");

        const __flags: string[] = [];

        if (options.color != null) {
            __flags.push("--color", "" + options.color);
        }

        if (options.disableIntegrityChecking) {
            __flags.push("--disable-integrity-checking");
        }

        if (options.fullyQualifyStackNames) {
            __flags.push("--fully-qualify-stack-names");
        }

        if (options.logflow) {
            __flags.push("--logflow");
        }

        if (options.logtostderr) {
            __flags.push("--logtostderr");
        }

        if (options.memprofilerate != null) {
            __flags.push("--memprofilerate", "" + options.memprofilerate);
        }

        if (options.otelTraces != null) {
            __flags.push("--otel-traces", "" + options.otelTraces);
        }

        if (options.profiling != null) {
            __flags.push("--profiling", "" + options.profiling);
        }

        if (options.tracing != null) {
            __flags.push("--tracing", "" + options.tracing);
        }

        if (options.tracingHeader != null) {
            __flags.push("--tracing-header", "" + options.tracingHeader);
        }

        if (options.verbose != null) {
            __flags.push("--verbose", "" + options.verbose);
        }

        if (options.delimiter != null) {
            __flags.push("--delimiter", "" + options.delimiter);
        }

        if (options.org != null) {
            __flags.push("--org", "" + options.org);
        }

        if (options.output != null) {
            __flags.push("--output", "" + options.output);
        }

        for (const __item of options.query ?? []) {
            if (__item != null) {
                __flags.push("--query", "" + __item);
            }
        }

        if (options.web) {
            __flags.push("--web");
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    orgSetDefault(options: PulumiOrgSetDefaultOptions, name: string): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("org");
        __final.push("set-default");

        const __flags: string[] = [];

        if (options.color != null) {
            __flags.push("--color", "" + options.color);
        }

        if (options.disableIntegrityChecking) {
            __flags.push("--disable-integrity-checking");
        }

        if (options.fullyQualifyStackNames) {
            __flags.push("--fully-qualify-stack-names");
        }

        if (options.logflow) {
            __flags.push("--logflow");
        }

        if (options.logtostderr) {
            __flags.push("--logtostderr");
        }

        if (options.memprofilerate != null) {
            __flags.push("--memprofilerate", "" + options.memprofilerate);
        }

        if (options.otelTraces != null) {
            __flags.push("--otel-traces", "" + options.otelTraces);
        }

        if (options.profiling != null) {
            __flags.push("--profiling", "" + options.profiling);
        }

        if (options.tracing != null) {
            __flags.push("--tracing", "" + options.tracing);
        }

        if (options.tracingHeader != null) {
            __flags.push("--tracing-header", "" + options.tracingHeader);
        }

        if (options.verbose != null) {
            __flags.push("--verbose", "" + options.verbose);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        __arguments.push("" + name);
        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    org(options: PulumiOrgOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("org");

        const __flags: string[] = [];

        if (options.color != null) {
            __flags.push("--color", "" + options.color);
        }

        if (options.disableIntegrityChecking) {
            __flags.push("--disable-integrity-checking");
        }

        if (options.fullyQualifyStackNames) {
            __flags.push("--fully-qualify-stack-names");
        }

        if (options.logflow) {
            __flags.push("--logflow");
        }

        if (options.logtostderr) {
            __flags.push("--logtostderr");
        }

        if (options.memprofilerate != null) {
            __flags.push("--memprofilerate", "" + options.memprofilerate);
        }

        if (options.otelTraces != null) {
            __flags.push("--otel-traces", "" + options.otelTraces);
        }

        if (options.profiling != null) {
            __flags.push("--profiling", "" + options.profiling);
        }

        if (options.tracing != null) {
            __flags.push("--tracing", "" + options.tracing);
        }

        if (options.tracingHeader != null) {
            __flags.push("--tracing-header", "" + options.tracingHeader);
        }

        if (options.verbose != null) {
            __flags.push("--verbose", "" + options.verbose);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }
}

/** Options for the `pulumi cancel` command. */
export interface PulumiCancelOptions extends BaseOptions {
    /** Colorize output. Choices are: always, never, raw, auto */
    color?: string;
    /** Disable integrity checking of checkpoint files */
    disableIntegrityChecking?: boolean;
    /** Show fully-qualified stack names */
    fullyQualifyStackNames?: boolean;
    /** Flow log settings to child processes (like plugins) */
    logflow?: boolean;
    /** Log to stderr instead of to files */
    logtostderr?: boolean;
    /** Enable more precise (and expensive) memory allocation profiles by setting runtime.MemProfileRate */
    memprofilerate?: number;
    /** Export OpenTelemetry traces to the specified endpoint. Use file:// for local JSON files, grpc:// for remote collectors */
    otelTraces?: string;
    /** Emit CPU and memory profiles and an execution trace to '[filename].[pid].{cpu,mem,trace}', respectively */
    profiling?: string;
    /** Emit tracing to the specified endpoint. Use the `file:` scheme to write tracing data to a local file */
    tracing?: string;
    /** Include the tracing header with the given contents. */
    tracingHeader?: string;
    /** Enable verbose logging (e.g., v=3); anything >3 is very verbose */
    verbose?: number;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi new` command. */
export interface PulumiNewOptions extends BaseOptions {
    /** Colorize output. Choices are: always, never, raw, auto */
    color?: string;
    /** Disable integrity checking of checkpoint files */
    disableIntegrityChecking?: boolean;
    /** Show fully-qualified stack names */
    fullyQualifyStackNames?: boolean;
    /** Flow log settings to child processes (like plugins) */
    logflow?: boolean;
    /** Log to stderr instead of to files */
    logtostderr?: boolean;
    /** Enable more precise (and expensive) memory allocation profiles by setting runtime.MemProfileRate */
    memprofilerate?: number;
    /** Export OpenTelemetry traces to the specified endpoint. Use file:// for local JSON files, grpc:// for remote collectors */
    otelTraces?: string;
    /** Emit CPU and memory profiles and an execution trace to '[filename].[pid].{cpu,mem,trace}', respectively */
    profiling?: string;
    /** Emit tracing to the specified endpoint. Use the `file:` scheme to write tracing data to a local file */
    tracing?: string;
    /** Include the tracing header with the given contents. */
    tracingHeader?: string;
    /** Enable verbose logging (e.g., v=3); anything >3 is very verbose */
    verbose?: number;
    /** Prompt to use for Pulumi AI */
    ai?: string;
    /** Config to save */
    config?: string[];
    /** Config keys contain a path to a property in a map or list to set */
    configPath?: boolean;
    /** The project description; if not specified, a prompt will request it */
    description?: string;
    /** The location to place the generated project; if not specified, the current directory is used */
    dir?: string;
    /** Forces content to be generated even if it would change existing files */
    force?: boolean;
    /** Generate the project only; do not create a stack, save config, or install dependencies */
    generateOnly?: boolean;
    /** Language to use for Pulumi AI (must be one of TypeScript, JavaScript, Python, Go, C#, Java, or YAML) */
    language?: string;
    /** List locally installed templates and exit */
    listTemplates?: boolean;
    /** The project name; if not specified, a prompt will request it */
    name?: string;
    /** Use locally cached templates without making any network requests */
    offline?: boolean;
    /** Store stack configuration remotely */
    remoteStackConfig?: boolean;
    /** Additional options for the language runtime (format: key1=value1,key2=value2) */
    runtimeOptions?: string[];
    /** The type of the provider that should be used to encrypt and decrypt secrets (possible choices: default, passphrase, awskms, azurekeyvault, gcpkms, hashivault) */
    secretsProvider?: string;
    /** The stack name; either an existing stack or stack to create; if not specified, a prompt will request it */
    stack?: string;
    /** Run in template mode, which will skip prompting for AI or Template functionality */
    templateMode?: boolean;
}

/** Options for the `pulumi org` command. */
export interface PulumiOrgOptions extends BaseOptions {
    /** Colorize output. Choices are: always, never, raw, auto */
    color?: string;
    /** Disable integrity checking of checkpoint files */
    disableIntegrityChecking?: boolean;
    /** Show fully-qualified stack names */
    fullyQualifyStackNames?: boolean;
    /** Flow log settings to child processes (like plugins) */
    logflow?: boolean;
    /** Log to stderr instead of to files */
    logtostderr?: boolean;
    /** Enable more precise (and expensive) memory allocation profiles by setting runtime.MemProfileRate */
    memprofilerate?: number;
    /** Export OpenTelemetry traces to the specified endpoint. Use file:// for local JSON files, grpc:// for remote collectors */
    otelTraces?: string;
    /** Emit CPU and memory profiles and an execution trace to '[filename].[pid].{cpu,mem,trace}', respectively */
    profiling?: string;
    /** Emit tracing to the specified endpoint. Use the `file:` scheme to write tracing data to a local file */
    tracing?: string;
    /** Include the tracing header with the given contents. */
    tracingHeader?: string;
    /** Enable verbose logging (e.g., v=3); anything >3 is very verbose */
    verbose?: number;
}

/** Options for the `pulumi org get-default` command. */
export interface PulumiOrgGetDefaultOptions extends BaseOptions {
    /** Colorize output. Choices are: always, never, raw, auto */
    color?: string;
    /** Disable integrity checking of checkpoint files */
    disableIntegrityChecking?: boolean;
    /** Show fully-qualified stack names */
    fullyQualifyStackNames?: boolean;
    /** Flow log settings to child processes (like plugins) */
    logflow?: boolean;
    /** Log to stderr instead of to files */
    logtostderr?: boolean;
    /** Enable more precise (and expensive) memory allocation profiles by setting runtime.MemProfileRate */
    memprofilerate?: number;
    /** Export OpenTelemetry traces to the specified endpoint. Use file:// for local JSON files, grpc:// for remote collectors */
    otelTraces?: string;
    /** Emit CPU and memory profiles and an execution trace to '[filename].[pid].{cpu,mem,trace}', respectively */
    profiling?: string;
    /** Emit tracing to the specified endpoint. Use the `file:` scheme to write tracing data to a local file */
    tracing?: string;
    /** Include the tracing header with the given contents. */
    tracingHeader?: string;
    /** Enable verbose logging (e.g., v=3); anything >3 is very verbose */
    verbose?: number;
}

/** Options for the `pulumi org search` command. */
export interface PulumiOrgSearchOptions extends BaseOptions {
    /** Colorize output. Choices are: always, never, raw, auto */
    color?: string;
    /** Disable integrity checking of checkpoint files */
    disableIntegrityChecking?: boolean;
    /** Show fully-qualified stack names */
    fullyQualifyStackNames?: boolean;
    /** Flow log settings to child processes (like plugins) */
    logflow?: boolean;
    /** Log to stderr instead of to files */
    logtostderr?: boolean;
    /** Enable more precise (and expensive) memory allocation profiles by setting runtime.MemProfileRate */
    memprofilerate?: number;
    /** Export OpenTelemetry traces to the specified endpoint. Use file:// for local JSON files, grpc:// for remote collectors */
    otelTraces?: string;
    /** Emit CPU and memory profiles and an execution trace to '[filename].[pid].{cpu,mem,trace}', respectively */
    profiling?: string;
    /** Emit tracing to the specified endpoint. Use the `file:` scheme to write tracing data to a local file */
    tracing?: string;
    /** Include the tracing header with the given contents. */
    tracingHeader?: string;
    /** Enable verbose logging (e.g., v=3); anything >3 is very verbose */
    verbose?: number;
    /** Delimiter to use when rendering CSV output. */
    delimiter?: string;
    /** Name of the organization to search. Defaults to the current user's default organization. */
    org?: string;
    /** Output format. Supported formats are 'table', 'json', 'csv', and 'yaml'. */
    output?: string;
    /**
     * A Pulumi Query to send to Pulumi Cloud for resource search.May be formatted as a single query, or multiple:
     * 	-q "type:aws:s3/bucketv2:BucketV2 modified:>=2023-09-01"
     * 	-q "type:aws:s3/bucketv2:BucketV2" -q "modified:>=2023-09-01"
     * See https://www.pulumi.com/docs/pulumi-cloud/insights/search/#query-syntax for syntax reference.
     */
    query?: string[];
    /** Open the search results in a web browser. */
    web?: boolean;
}

/** Options for the `pulumi org search ai` command. */
export interface PulumiOrgSearchAiOptions extends BaseOptions {
    /** Colorize output. Choices are: always, never, raw, auto */
    color?: string;
    /** Disable integrity checking of checkpoint files */
    disableIntegrityChecking?: boolean;
    /** Show fully-qualified stack names */
    fullyQualifyStackNames?: boolean;
    /** Flow log settings to child processes (like plugins) */
    logflow?: boolean;
    /** Log to stderr instead of to files */
    logtostderr?: boolean;
    /** Enable more precise (and expensive) memory allocation profiles by setting runtime.MemProfileRate */
    memprofilerate?: number;
    /** Export OpenTelemetry traces to the specified endpoint. Use file:// for local JSON files, grpc:// for remote collectors */
    otelTraces?: string;
    /** Emit CPU and memory profiles and an execution trace to '[filename].[pid].{cpu,mem,trace}', respectively */
    profiling?: string;
    /** Emit tracing to the specified endpoint. Use the `file:` scheme to write tracing data to a local file */
    tracing?: string;
    /** Include the tracing header with the given contents. */
    tracingHeader?: string;
    /** Enable verbose logging (e.g., v=3); anything >3 is very verbose */
    verbose?: number;
    /** Delimiter to use when rendering CSV output. */
    delimiter?: string;
    /** Organization name to search within */
    org?: string;
    /** Output format. Supported formats are 'table', 'json', 'csv' and 'yaml'. */
    output?: string;
    /** Plaintext natural language query */
    query?: string;
    /** Open the search results in a web browser. */
    web?: boolean;
}

/** Options for the `pulumi org set-default` command. */
export interface PulumiOrgSetDefaultOptions extends BaseOptions {
    /** Colorize output. Choices are: always, never, raw, auto */
    color?: string;
    /** Disable integrity checking of checkpoint files */
    disableIntegrityChecking?: boolean;
    /** Show fully-qualified stack names */
    fullyQualifyStackNames?: boolean;
    /** Flow log settings to child processes (like plugins) */
    logflow?: boolean;
    /** Log to stderr instead of to files */
    logtostderr?: boolean;
    /** Enable more precise (and expensive) memory allocation profiles by setting runtime.MemProfileRate */
    memprofilerate?: number;
    /** Export OpenTelemetry traces to the specified endpoint. Use file:// for local JSON files, grpc:// for remote collectors */
    otelTraces?: string;
    /** Emit CPU and memory profiles and an execution trace to '[filename].[pid].{cpu,mem,trace}', respectively */
    profiling?: string;
    /** Emit tracing to the specified endpoint. Use the `file:` scheme to write tracing data to a local file */
    tracing?: string;
    /** Include the tracing header with the given contents. */
    tracingHeader?: string;
    /** Enable verbose logging (e.g., v=3); anything >3 is very verbose */
    verbose?: number;
}
