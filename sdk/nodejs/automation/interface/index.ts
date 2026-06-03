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

    envClone(options: PulumiEnvCloneOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("clone");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.preserveAccess) {
            __flags.push("--preserve-access");
        }

        if (options.preserveEnvTags) {
            __flags.push("--preserve-env-tags");
        }

        if (options.preserveHistory) {
            __flags.push("--preserve-history");
        }

        if (options.preserveRevTags) {
            __flags.push("--preserve-rev-tags");
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envDiff(options: PulumiEnvDiffOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("diff");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.format != null) {
            __flags.push("--format", "" + options.format);
        }

        if (options.path != null) {
            __flags.push("--path", "" + options.path);
        }

        if (options.showSecrets) {
            __flags.push("--show-secrets");
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envEdit(options: PulumiEnvEditOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("edit");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.draft != null) {
            __flags.push("--draft", "" + options.draft);
        }

        if (options.editor != null) {
            __flags.push("--editor", "" + options.editor);
        }

        if (options.file != null) {
            __flags.push("--file", "" + options.file);
        }

        if (options.showSecrets) {
            __flags.push("--show-secrets");
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envGet(options: PulumiEnvGetOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("get");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.definition) {
            __flags.push("--definition");
        }

        if (options.showSecrets) {
            __flags.push("--show-secrets");
        }

        if (options.value != null) {
            __flags.push("--value", "" + options.value);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envInit(options: PulumiEnvInitOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("init");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.file != null) {
            __flags.push("--file", "" + options.file);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envLs(options: PulumiEnvLsOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("ls");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.organization != null) {
            __flags.push("--organization", "" + options.organization);
        }

        if (options.output != null) {
            __flags.push("--output", "" + options.output);
        }

        if (options.project != null) {
            __flags.push("--project", "" + options.project);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envOpen(options: PulumiEnvOpenOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("open");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.draft != null) {
            __flags.push("--draft", "" + options.draft);
        }

        if (options.format != null) {
            __flags.push("--format", "" + options.format);
        }

        if (options.lifetime != null) {
            __flags.push("--lifetime", "" + options.lifetime);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envOpenRequest(options: PulumiEnvOpenRequestOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("open-request");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.accessDurationSeconds != null) {
            __flags.push("--access-duration-seconds", "" + options.accessDurationSeconds);
        }

        if (options.grantExpirationSeconds != null) {
            __flags.push("--grant-expiration-seconds", "" + options.grantExpirationSeconds);
        }

        if (options.output != null) {
            __flags.push("--output", "" + options.output);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envProviderAwsLoginOidc(options: PulumiEnvProviderAwsLoginOidcOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("provider");
        __final.push("aws-login");
        __final.push("oidc");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.create) {
            __flags.push("--create");
        }

        if (options.draft != null) {
            __flags.push("--draft", "" + options.draft);
        }

        if (options.duration != null) {
            __flags.push("--duration", "" + options.duration);
        }

        if (options.path != null) {
            __flags.push("--path", "" + options.path);
        }

        for (const __item of options.policyArn ?? []) {
            if (__item != null) {
                __flags.push("--policy-arn", "" + __item);
            }
        }

        for (const __item of options.subjectAttribute ?? []) {
            if (__item != null) {
                __flags.push("--subject-attribute", "" + __item);
            }
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envProviderAwsLoginStatic(options: PulumiEnvProviderAwsLoginStaticOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("provider");
        __final.push("aws-login");
        __final.push("static");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.create) {
            __flags.push("--create");
        }

        if (options.draft != null) {
            __flags.push("--draft", "" + options.draft);
        }

        if (options.path != null) {
            __flags.push("--path", "" + options.path);
        }

        if (options.sessionToken != null) {
            __flags.push("--session-token", "" + options.sessionToken);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envProviderAzureLoginOidc(options: PulumiEnvProviderAzureLoginOidcOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("provider");
        __final.push("azure-login");
        __final.push("oidc");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.create) {
            __flags.push("--create");
        }

        if (options.draft != null) {
            __flags.push("--draft", "" + options.draft);
        }

        if (options.path != null) {
            __flags.push("--path", "" + options.path);
        }

        for (const __item of options.subjectAttribute ?? []) {
            if (__item != null) {
                __flags.push("--subject-attribute", "" + __item);
            }
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envProviderAzureLoginStatic(options: PulumiEnvProviderAzureLoginStaticOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("provider");
        __final.push("azure-login");
        __final.push("static");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.create) {
            __flags.push("--create");
        }

        if (options.draft != null) {
            __flags.push("--draft", "" + options.draft);
        }

        if (options.path != null) {
            __flags.push("--path", "" + options.path);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envProviderGcpLoginOidc(options: PulumiEnvProviderGcpLoginOidcOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("provider");
        __final.push("gcp-login");
        __final.push("oidc");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.create) {
            __flags.push("--create");
        }

        if (options.draft != null) {
            __flags.push("--draft", "" + options.draft);
        }

        if (options.path != null) {
            __flags.push("--path", "" + options.path);
        }

        __flags.push("--provider-id", "" + options.providerId);

        if (options.region != null) {
            __flags.push("--region", "" + options.region);
        }

        __flags.push("--service-account", "" + options.serviceAccount);

        for (const __item of options.subjectAttribute ?? []) {
            if (__item != null) {
                __flags.push("--subject-attribute", "" + __item);
            }
        }

        if (options.tokenLifetime != null) {
            __flags.push("--token-lifetime", "" + options.tokenLifetime);
        }

        __flags.push("--workload-pool-id", "" + options.workloadPoolId);

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envProviderGcpLoginStatic(options: PulumiEnvProviderGcpLoginStaticOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("provider");
        __final.push("gcp-login");
        __final.push("static");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.create) {
            __flags.push("--create");
        }

        if (options.draft != null) {
            __flags.push("--draft", "" + options.draft);
        }

        if (options.path != null) {
            __flags.push("--path", "" + options.path);
        }

        if (options.serviceAccount != null) {
            __flags.push("--service-account", "" + options.serviceAccount);
        }

        if (options.tokenLifetime != null) {
            __flags.push("--token-lifetime", "" + options.tokenLifetime);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envReferrerList(options: PulumiEnvReferrerListOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("referrer");
        __final.push("list");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.all) {
            __flags.push("--all");
        }

        if (options.allRevisions) {
            __flags.push("--all-revisions");
        }

        if (options.count != null) {
            __flags.push("--count", "" + options.count);
        }

        if (options.latestStackVersionOnly) {
            __flags.push("--latest-stack-version-only");
        }

        if (options.output != null) {
            __flags.push("--output", "" + options.output);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envRm(options: PulumiEnvRmOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("rm");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envRotate(options: PulumiEnvRotateOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("rotate");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envRun(options: PulumiEnvRunOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("run");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.draft != null) {
            __flags.push("--draft", "" + options.draft);
        }

        if (options.interactive) {
            __flags.push("--interactive");
        }

        if (options.lifetime != null) {
            __flags.push("--lifetime", "" + options.lifetime);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envScheduleEdit(options: PulumiEnvScheduleEditOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("schedule");
        __final.push("edit");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.cron != null) {
            __flags.push("--cron", "" + options.cron);
        }

        if (options.once != null) {
            __flags.push("--once", "" + options.once);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envScheduleGet(options: PulumiEnvScheduleGetOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("schedule");
        __final.push("get");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.output != null) {
            __flags.push("--output", "" + options.output);
        }

        if (options.utc) {
            __flags.push("--utc");
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envScheduleHistory(options: PulumiEnvScheduleHistoryOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("schedule");
        __final.push("history");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.count != null) {
            __flags.push("--count", "" + options.count);
        }

        if (options.output != null) {
            __flags.push("--output", "" + options.output);
        }

        if (options.utc) {
            __flags.push("--utc");
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envScheduleList(options: PulumiEnvScheduleListOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("schedule");
        __final.push("list");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.count != null) {
            __flags.push("--count", "" + options.count);
        }

        if (options.output != null) {
            __flags.push("--output", "" + options.output);
        }

        if (options.utc) {
            __flags.push("--utc");
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envScheduleNew(options: PulumiEnvScheduleNewOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("schedule");
        __final.push("new");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.cron != null) {
            __flags.push("--cron", "" + options.cron);
        }

        if (options.once != null) {
            __flags.push("--once", "" + options.once);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envScheduleRemove(options: PulumiEnvScheduleRemoveOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("schedule");
        __final.push("remove");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envSet(options: PulumiEnvSetOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("set");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.draft != null) {
            __flags.push("--draft", "" + options.draft);
        }

        if (options.file != null) {
            __flags.push("--file", "" + options.file);
        }

        if (options.plaintext) {
            __flags.push("--plaintext");
        }

        if (options.secret) {
            __flags.push("--secret");
        }

        if (options.string) {
            __flags.push("--string");
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envSettingsGet(options: PulumiEnvSettingsGetOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("settings");
        __final.push("get");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.output != null) {
            __flags.push("--output", "" + options.output);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envSettingsSet(options: PulumiEnvSettingsSetOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("settings");
        __final.push("set");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envTagGet(options: PulumiEnvTagGetOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("tag");
        __final.push("get");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.utc) {
            __flags.push("--utc");
        }

        if (options.output != null) {
            __flags.push("--output", "" + options.output);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envTagLs(options: PulumiEnvTagLsOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("tag");
        __final.push("ls");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.utc) {
            __flags.push("--utc");
        }

        if (options.output != null) {
            __flags.push("--output", "" + options.output);
        }

        if (options.pager != null) {
            __flags.push("--pager", "" + options.pager);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envTagMv(options: PulumiEnvTagMvOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("tag");
        __final.push("mv");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.utc) {
            __flags.push("--utc");
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envTagRm(options: PulumiEnvTagRmOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("tag");
        __final.push("rm");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.utc) {
            __flags.push("--utc");
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envTag(options: PulumiEnvTagOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("tag");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.utc) {
            __flags.push("--utc");
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envVersionHistory(options: PulumiEnvVersionHistoryOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("version");
        __final.push("history");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.utc) {
            __flags.push("--utc");
        }

        if (options.output != null) {
            __flags.push("--output", "" + options.output);
        }

        if (options.pager != null) {
            __flags.push("--pager", "" + options.pager);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envVersionRetract(options: PulumiEnvVersionRetractOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("version");
        __final.push("retract");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.utc) {
            __flags.push("--utc");
        }

        if (options.reason != null) {
            __flags.push("--reason", "" + options.reason);
        }

        if (options.replaceWith != null) {
            __flags.push("--replace-with", "" + options.replaceWith);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envVersionRollback(options: PulumiEnvVersionRollbackOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("version");
        __final.push("rollback");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.utc) {
            __flags.push("--utc");
        }

        if (options.draft != null) {
            __flags.push("--draft", "" + options.draft);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envVersionTagLs(options: PulumiEnvVersionTagLsOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("version");
        __final.push("tag");
        __final.push("ls");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.utc) {
            __flags.push("--utc");
        }

        if (options.output != null) {
            __flags.push("--output", "" + options.output);
        }

        if (options.pager != null) {
            __flags.push("--pager", "" + options.pager);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envVersionTagRm(options: PulumiEnvVersionTagRmOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("version");
        __final.push("tag");
        __final.push("rm");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.utc) {
            __flags.push("--utc");
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envVersionTag(options: PulumiEnvVersionTagOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("version");
        __final.push("tag");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.utc) {
            __flags.push("--utc");
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envVersion(options: PulumiEnvVersionOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("version");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.utc) {
            __flags.push("--utc");
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envWebhookDeliveryList(options: PulumiEnvWebhookDeliveryListOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("webhook");
        __final.push("delivery");
        __final.push("list");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.count != null) {
            __flags.push("--count", "" + options.count);
        }

        if (options.output != null) {
            __flags.push("--output", "" + options.output);
        }

        if (options.utc) {
            __flags.push("--utc");
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envWebhookEdit(options: PulumiEnvWebhookEditOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("webhook");
        __final.push("edit");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.active) {
            __flags.push("--active");
        }

        for (const __item of options.addEvent ?? []) {
            if (__item != null) {
                __flags.push("--add-event", "" + __item);
            }
        }

        for (const __item of options.addGroup ?? []) {
            if (__item != null) {
                __flags.push("--add-group", "" + __item);
            }
        }

        if (options.displayName != null) {
            __flags.push("--display-name", "" + options.displayName);
        }

        for (const __item of options.event ?? []) {
            if (__item != null) {
                __flags.push("--event", "" + __item);
            }
        }

        if (options.format != null) {
            __flags.push("--format", "" + options.format);
        }

        for (const __item of options.group ?? []) {
            if (__item != null) {
                __flags.push("--group", "" + __item);
            }
        }

        for (const __item of options.removeEvent ?? []) {
            if (__item != null) {
                __flags.push("--remove-event", "" + __item);
            }
        }

        for (const __item of options.removeGroup ?? []) {
            if (__item != null) {
                __flags.push("--remove-group", "" + __item);
            }
        }

        if (options.removeSecret) {
            __flags.push("--remove-secret");
        }

        if (options.secret != null) {
            __flags.push("--secret", "" + options.secret);
        }

        if (options.url != null) {
            __flags.push("--url", "" + options.url);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envWebhookGet(options: PulumiEnvWebhookGetOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("webhook");
        __final.push("get");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.output != null) {
            __flags.push("--output", "" + options.output);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envWebhookList(options: PulumiEnvWebhookListOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("webhook");
        __final.push("list");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.count != null) {
            __flags.push("--count", "" + options.count);
        }

        if (options.output != null) {
            __flags.push("--output", "" + options.output);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envWebhookNew(options: PulumiEnvWebhookNewOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("webhook");
        __final.push("new");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.active) {
            __flags.push("--active");
        }

        for (const __item of options.event ?? []) {
            if (__item != null) {
                __flags.push("--event", "" + __item);
            }
        }

        if (options.format != null) {
            __flags.push("--format", "" + options.format);
        }

        for (const __item of options.group ?? []) {
            if (__item != null) {
                __flags.push("--group", "" + __item);
            }
        }

        if (options.secret != null) {
            __flags.push("--secret", "" + options.secret);
        }

        if (options.url != null) {
            __flags.push("--url", "" + options.url);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envWebhookPing(options: PulumiEnvWebhookPingOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("webhook");
        __final.push("ping");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envWebhookRm(options: PulumiEnvWebhookRmOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("webhook");
        __final.push("rm");

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

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

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

/** Options for the `pulumi env clone` command. */
export interface PulumiEnvCloneOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** preserve the same team access on the environment being cloned */
    preserveAccess?: boolean;
    /** preserve any tags on the environment being cloned */
    preserveEnvTags?: boolean;
    /** preserve history of the environment being cloned */
    preserveHistory?: boolean;
    /** preserve any tags on the environment revisions being cloned */
    preserveRevTags?: boolean;
}

/** Options for the `pulumi env diff` command. */
export interface PulumiEnvDiffOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** the output format to use. May be 'dotenv', 'json', 'yaml', 'detailed', or 'shell' */
    format?: string;
    /** Show the diff for a specific path */
    path?: string;
    /** Show static secrets in plaintext rather than ciphertext */
    showSecrets?: boolean;
}

/** Options for the `pulumi env edit` command. */
export interface PulumiEnvEditOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** set flag without a value (--draft) to create a draft rather than saving changes directly. --draft=<change-request-id> to update an existing change request. */
    draft?: string;
    /** the command to use to edit the environment definition */
    editor?: string;
    /** the file that contains the updated environment, if any. Pass `-` to read from standard input. */
    file?: string;
    /** Show static secrets in plaintext rather than ciphertext */
    showSecrets?: boolean;
}

/** Options for the `pulumi env get` command. */
export interface PulumiEnvGetOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** Set to print just the definition. */
    definition?: boolean;
    /** Show static secrets in plaintext rather than ciphertext */
    showSecrets?: boolean;
    /** Set to print just the value in the given format. May be 'dotenv', 'json', 'detailed', 'shell' or 'string' */
    value?: string;
}

/** Options for the `pulumi env init` command. */
export interface PulumiEnvInitOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** the file to use to initialize the environment, if any. Pass `-` to read from standard input. */
    file?: string;
}

/** Options for the `pulumi env ls` command. */
export interface PulumiEnvLsOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** Filter returned environments to those in a specific organization */
    organization?: string;
    /** output format: "text" (default) or "json" */
    output?: string;
    /** Filter returned environments to those in a specific project */
    project?: string;
}

/** Options for the `pulumi env open` command. */
export interface PulumiEnvOpenOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** open an environment draft with --draft=<change-request-id> */
    draft?: string;
    /** the output format to use. May be 'dotenv', 'json', 'yaml', 'detailed', 'shell' or 'string' */
    format?: string;
    /** the lifetime of the opened environment in the form HhMm (e.g. 2h, 1h30m, 15m) */
    lifetime?: string;
}

/** Options for the `pulumi env open-request` command. */
export interface PulumiEnvOpenRequestOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** duration of access in seconds */
    accessDurationSeconds?: string;
    /** expiration time for the grant in seconds */
    grantExpirationSeconds?: string;
    /** output format: "text" (default) or "json" */
    output?: string;
}

/** Options for the `pulumi env provider aws-login oidc` command. */
export interface PulumiEnvProviderAwsLoginOidcOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** create the environment if it does not already exist */
    create?: boolean;
    /** set flag without a value (--draft) to create a draft rather than saving changes directly. --draft=<change-request-id> to update an existing change request. */
    draft?: string;
    /** optional session duration, e.g. 1h */
    duration?: string;
    /** property path under `values` where the provider block is written */
    path?: string;
    /** AWS managed-policy ARN to attach to the role session (repeatable) */
    policyArn?: string[];
    /** OIDC subject attribute to include in the session token (repeatable) */
    subjectAttribute?: string[];
}

/** Options for the `pulumi env provider aws-login static` command. */
export interface PulumiEnvProviderAwsLoginStaticOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** create the environment if it does not already exist */
    create?: boolean;
    /** set flag without a value (--draft) to create a draft rather than saving changes directly. --draft=<change-request-id> to update an existing change request. */
    draft?: string;
    /** property path under `values` where the provider block is written */
    path?: string;
    /** optional AWS session token */
    sessionToken?: string;
}

/** Options for the `pulumi env provider azure-login oidc` command. */
export interface PulumiEnvProviderAzureLoginOidcOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** create the environment if it does not already exist */
    create?: boolean;
    /** set flag without a value (--draft) to create a draft rather than saving changes directly. --draft=<change-request-id> to update an existing change request. */
    draft?: string;
    /** property path under `values` where the provider block is written */
    path?: string;
    /** OIDC subject attribute to include in the federated token (repeatable) */
    subjectAttribute?: string[];
}

/** Options for the `pulumi env provider azure-login static` command. */
export interface PulumiEnvProviderAzureLoginStaticOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** create the environment if it does not already exist */
    create?: boolean;
    /** set flag without a value (--draft) to create a draft rather than saving changes directly. --draft=<change-request-id> to update an existing change request. */
    draft?: string;
    /** property path under `values` where the provider block is written */
    path?: string;
}

/** Options for the `pulumi env provider gcp-login oidc` command. */
export interface PulumiEnvProviderGcpLoginOidcOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** create the environment if it does not already exist */
    create?: boolean;
    /** set flag without a value (--draft) to create a draft rather than saving changes directly. --draft=<change-request-id> to update an existing change request. */
    draft?: string;
    /** property path under `values` where the provider block is written */
    path?: string;
    /** GCP workload identity pool provider ID (required) */
    providerId: string;
    /** optional GCP region for the workload identity pool */
    region?: string;
    /** GCP service account to impersonate (required) */
    serviceAccount: string;
    /** OIDC subject attribute to include in the federated token (repeatable) */
    subjectAttribute?: string[];
    /** optional lifetime for impersonated credentials, e.g. 1h30m */
    tokenLifetime?: string;
    /** GCP workload identity pool ID (required) */
    workloadPoolId: string;
}

/** Options for the `pulumi env provider gcp-login static` command. */
export interface PulumiEnvProviderGcpLoginStaticOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** create the environment if it does not already exist */
    create?: boolean;
    /** set flag without a value (--draft) to create a draft rather than saving changes directly. --draft=<change-request-id> to update an existing change request. */
    draft?: string;
    /** property path under `values` where the provider block is written */
    path?: string;
    /** optional GCP service account to impersonate */
    serviceAccount?: string;
    /** optional lifetime for impersonated credentials, e.g. 1h30m */
    tokenLifetime?: string;
}

/** Options for the `pulumi env referrer list` command. */
export interface PulumiEnvReferrerListOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** return all referrers, paginating through every page. Mutually exclusive with --count */
    all?: boolean;
    /** include referrers across all revisions of the environment, not just the latest */
    allRevisions?: boolean;
    /** the maximum number of referrers to return (server default if unset; max 500). Mutually exclusive with --all */
    count?: number;
    /** only include the latest version of each referring stack */
    latestStackVersionOnly?: boolean;
    /** output format: "text" (default) or "json" */
    output?: string;
}

/** Options for the `pulumi env rm` command. */
export interface PulumiEnvRmOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
}

/** Options for the `pulumi env rotate` command. */
export interface PulumiEnvRotateOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
}

/** Options for the `pulumi env run` command. */
export interface PulumiEnvRunOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** open an environment draft with --draft=<change-request-id> */
    draft?: string;
    /** true to treat the command as interactive and disable output filters */
    interactive?: boolean;
    /** the lifetime of the opened environment */
    lifetime?: string;
}

/** Options for the `pulumi env schedule edit` command. */
export interface PulumiEnvScheduleEditOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** a cron expression for a recurring schedule (minimum interval: once daily) */
    cron?: string;
    /** an ISO 8601 / RFC 3339 timestamp in the future for a one-time schedule */
    once?: string;
}

/** Options for the `pulumi env schedule get` command. */
export interface PulumiEnvScheduleGetOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** output format: "text" (default) or "json" */
    output?: string;
    /** display times in UTC */
    utc?: boolean;
}

/** Options for the `pulumi env schedule history` command. */
export interface PulumiEnvScheduleHistoryOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** the maximum number of events to return (all if unset) */
    count?: number;
    /** output format: "text" (default) or "json" */
    output?: string;
    /** display times in UTC */
    utc?: boolean;
}

/** Options for the `pulumi env schedule list` command. */
export interface PulumiEnvScheduleListOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** the maximum number of schedules to return (all if unset) */
    count?: number;
    /** output format: "text" (default) or "json" */
    output?: string;
    /** display times in UTC */
    utc?: boolean;
}

/** Options for the `pulumi env schedule new` command. */
export interface PulumiEnvScheduleNewOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** a cron expression for a recurring schedule (minimum interval: once daily) */
    cron?: string;
    /** an ISO 8601 / RFC 3339 timestamp in the future for a one-time schedule */
    once?: string;
}

/** Options for the `pulumi env schedule remove` command. */
export interface PulumiEnvScheduleRemoveOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
}

/** Options for the `pulumi env set` command. */
export interface PulumiEnvSetOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** set flag without a value (--draft) to create a draft rather than saving changes directly. --draft=<change-request-id> to update an existing change request. */
    draft?: string;
    /** If set, the value is read from the specified file. Pass `-` to read from standard input. */
    file?: string;
    /** true to leave the value in plaintext */
    plaintext?: boolean;
    /** true to mark the value as secret */
    secret?: boolean;
    /** true to treat the value as a string rather than attempting to parse it as YAML */
    string?: boolean;
}

/** Options for the `pulumi env settings get` command. */
export interface PulumiEnvSettingsGetOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** output format: "text" (default) or "json" */
    output?: string;
}

/** Options for the `pulumi env settings set` command. */
export interface PulumiEnvSettingsSetOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
}

/** Options for the `pulumi env tag` command. */
export interface PulumiEnvTagOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** display times in UTC */
    utc?: boolean;
}

/** Options for the `pulumi env tag get` command. */
export interface PulumiEnvTagGetOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** display times in UTC */
    utc?: boolean;
    /** output format: "text" (default) or "json" */
    output?: string;
}

/** Options for the `pulumi env tag ls` command. */
export interface PulumiEnvTagLsOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** display times in UTC */
    utc?: boolean;
    /** output format: "text" (default) or "json" */
    output?: string;
    /** the command to use to page through the environment's version tags */
    pager?: string;
}

/** Options for the `pulumi env tag mv` command. */
export interface PulumiEnvTagMvOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** display times in UTC */
    utc?: boolean;
}

/** Options for the `pulumi env tag rm` command. */
export interface PulumiEnvTagRmOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** display times in UTC */
    utc?: boolean;
}

/** Options for the `pulumi env version` command. */
export interface PulumiEnvVersionOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** display times in UTC */
    utc?: boolean;
}

/** Options for the `pulumi env version history` command. */
export interface PulumiEnvVersionHistoryOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** display times in UTC */
    utc?: boolean;
    /** output format: "text" (default) or "json" */
    output?: string;
    /** the command to use to page through the environment's revisions */
    pager?: string;
}

/** Options for the `pulumi env version retract` command. */
export interface PulumiEnvVersionRetractOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** display times in UTC */
    utc?: boolean;
    /** the reason for the retraction */
    reason?: string;
    /** the version to use to replace the retracted revision */
    replaceWith?: string;
}

/** Options for the `pulumi env version rollback` command. */
export interface PulumiEnvVersionRollbackOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** display times in UTC */
    utc?: boolean;
    /** set flag without a value (--draft) to create a draft rather than saving changes directly. --draft=<change-request-id> to update an existing change request. */
    draft?: string;
}

/** Options for the `pulumi env version tag` command. */
export interface PulumiEnvVersionTagOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** display times in UTC */
    utc?: boolean;
}

/** Options for the `pulumi env version tag ls` command. */
export interface PulumiEnvVersionTagLsOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** display times in UTC */
    utc?: boolean;
    /** output format: "text" (default) or "json" */
    output?: string;
    /** the command to use to page through the environment's version tags */
    pager?: string;
}

/** Options for the `pulumi env version tag rm` command. */
export interface PulumiEnvVersionTagRmOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** display times in UTC */
    utc?: boolean;
}

/** Options for the `pulumi env webhook delivery list` command. */
export interface PulumiEnvWebhookDeliveryListOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** The maximum number of deliveries to return (all if unset) */
    count?: number;
    /** output format: "text" (default) or "json" */
    output?: string;
    /** Display times in UTC */
    utc?: boolean;
}

/** Options for the `pulumi env webhook edit` command. */
export interface PulumiEnvWebhookEditOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** Whether the webhook is active */
    active?: boolean;
    /** Subscribe to an additional event (repeatable) */
    addEvent?: string[];
    /** Subscribe to an additional event group (repeatable) */
    addGroup?: string[];
    /** The display name */
    displayName?: string;
    /** Replace the subscribed events (repeatable) */
    event?: string[];
    /** The payload format */
    format?: string;
    /** Replace the subscribed event groups (repeatable) */
    group?: string[];
    /** Unsubscribe from an event (repeatable) */
    removeEvent?: string[];
    /** Unsubscribe from an event group (repeatable) */
    removeGroup?: string[];
    /** Clear the existing shared secret */
    removeSecret?: boolean;
    /** Shared secret used to sign deliveries */
    secret?: string;
    /** The payload URL to deliver events to */
    url?: string;
}

/** Options for the `pulumi env webhook get` command. */
export interface PulumiEnvWebhookGetOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** output format: "text" (default) or "json" */
    output?: string;
}

/** Options for the `pulumi env webhook list` command. */
export interface PulumiEnvWebhookListOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** The maximum number of webhooks to return (all if unset) */
    count?: number;
    /** output format: "text" (default) or "json" */
    output?: string;
}

/** Options for the `pulumi env webhook new` command. */
export interface PulumiEnvWebhookNewOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** Whether the webhook is active */
    active?: boolean;
    /** Event types to subscribe to (repeatable) */
    event?: string[];
    /** The payload format */
    format?: string;
    /** Event groups to subscribe to (repeatable) */
    group?: string[];
    /** Shared secret used to sign deliveries */
    secret?: string;
    /** The payload URL to deliver events to (required) */
    url?: string;
}

/** Options for the `pulumi env webhook ping` command. */
export interface PulumiEnvWebhookPingOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
}

/** Options for the `pulumi env webhook rm` command. */
export interface PulumiEnvWebhookRmOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
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
    /** Output format. Supported values are: default, json, yaml and csv */
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
    /** Output format. Supported values are: default, json, yaml and csv */
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
