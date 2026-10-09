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

    configCopy(options: PulumiConfigCopyOptions, key?: string): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("config");
        __final.push("copy");

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

        if (options.configFile != null) {
            __flags.push("--config-file", "" + options.configFile);
        }

        if (options.json) {
            __flags.push("--json");
        }

        if (options.open) {
            __flags.push("--open");
        }

        if (options.showSecrets) {
            __flags.push("--show-secrets");
        }

        if (options.stack != null) {
            __flags.push("--stack", "" + options.stack);
        }

        if (options.dest != null) {
            __flags.push("--dest", "" + options.dest);
        }

        if (options.path) {
            __flags.push("--path");
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (key != null) {
            __arguments.push("" + key);
        }
        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    configEnvAdd(options: PulumiConfigEnvAddOptions, ...environmentName: string[]): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("config");
        __final.push("env");
        __final.push("add");

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

        if (options.configFile != null) {
            __flags.push("--config-file", "" + options.configFile);
        }

        if (options.json) {
            __flags.push("--json");
        }

        if (options.open) {
            __flags.push("--open");
        }

        if (options.showSecrets) {
            __flags.push("--show-secrets");
        }

        if (options.stack != null) {
            __flags.push("--stack", "" + options.stack);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        for (const __item of environmentName ?? []) {
            __arguments.push("" + __item);
        }
        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    configEnvInit(options: PulumiConfigEnvInitOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("config");
        __final.push("env");
        __final.push("init");

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

        if (options.configFile != null) {
            __flags.push("--config-file", "" + options.configFile);
        }

        if (options.json) {
            __flags.push("--json");
        }

        if (options.open) {
            __flags.push("--open");
        }

        if (options.showSecrets) {
            __flags.push("--show-secrets");
        }

        if (options.stack != null) {
            __flags.push("--stack", "" + options.stack);
        }

        if (options.env != null) {
            __flags.push("--env", "" + options.env);
        }

        if (options.keepConfig) {
            __flags.push("--keep-config");
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    configEnvList(options: PulumiConfigEnvListOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("config");
        __final.push("env");
        __final.push("list");

        const __flags: string[] = [];

        __flags.push("--output", "" + "json");

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

        if (options.configFile != null) {
            __flags.push("--config-file", "" + options.configFile);
        }

        if (options.open) {
            __flags.push("--open");
        }

        if (options.showSecrets) {
            __flags.push("--show-secrets");
        }

        if (options.stack != null) {
            __flags.push("--stack", "" + options.stack);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    configEnvRemove(options: PulumiConfigEnvRemoveOptions, environmentName: string): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("config");
        __final.push("env");
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

        if (options.configFile != null) {
            __flags.push("--config-file", "" + options.configFile);
        }

        if (options.json) {
            __flags.push("--json");
        }

        if (options.open) {
            __flags.push("--open");
        }

        if (options.showSecrets) {
            __flags.push("--show-secrets");
        }

        if (options.stack != null) {
            __flags.push("--stack", "" + options.stack);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        __arguments.push("" + environmentName);
        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    configGet(options: PulumiConfigGetOptions, key: string): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("config");
        __final.push("get");

        const __flags: string[] = [];

        __flags.push("--json");

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

        if (options.configFile != null) {
            __flags.push("--config-file", "" + options.configFile);
        }

        if (options.open) {
            __flags.push("--open");
        }

        if (options.showSecrets) {
            __flags.push("--show-secrets");
        }

        if (options.stack != null) {
            __flags.push("--stack", "" + options.stack);
        }

        if (options.path) {
            __flags.push("--path");
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        __arguments.push("" + key);
        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    configRefresh(options: PulumiConfigRefreshOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("config");
        __final.push("refresh");

        const __flags: string[] = [];

        __flags.push("--force");

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

        if (options.configFile != null) {
            __flags.push("--config-file", "" + options.configFile);
        }

        if (options.json) {
            __flags.push("--json");
        }

        if (options.open) {
            __flags.push("--open");
        }

        if (options.showSecrets) {
            __flags.push("--show-secrets");
        }

        if (options.stack != null) {
            __flags.push("--stack", "" + options.stack);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    configRemove(options: PulumiConfigRemoveOptions, key: string): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("config");
        __final.push("remove");

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

        if (options.configFile != null) {
            __flags.push("--config-file", "" + options.configFile);
        }

        if (options.json) {
            __flags.push("--json");
        }

        if (options.open) {
            __flags.push("--open");
        }

        if (options.showSecrets) {
            __flags.push("--show-secrets");
        }

        if (options.stack != null) {
            __flags.push("--stack", "" + options.stack);
        }

        if (options.path) {
            __flags.push("--path");
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        __arguments.push("" + key);
        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    configRemoveAll(options: PulumiConfigRemoveAllOptions, ...key: string[]): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("config");
        __final.push("remove-all");

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

        if (options.configFile != null) {
            __flags.push("--config-file", "" + options.configFile);
        }

        if (options.json) {
            __flags.push("--json");
        }

        if (options.open) {
            __flags.push("--open");
        }

        if (options.showSecrets) {
            __flags.push("--show-secrets");
        }

        if (options.stack != null) {
            __flags.push("--stack", "" + options.stack);
        }

        if (options.path) {
            __flags.push("--path");
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        for (const __item of key ?? []) {
            __arguments.push("" + __item);
        }
        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    configSet(options: PulumiConfigSetOptions, key: string, value?: string): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("config");
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

        if (options.configFile != null) {
            __flags.push("--config-file", "" + options.configFile);
        }

        if (options.json) {
            __flags.push("--json");
        }

        if (options.open) {
            __flags.push("--open");
        }

        if (options.showSecrets) {
            __flags.push("--show-secrets");
        }

        if (options.stack != null) {
            __flags.push("--stack", "" + options.stack);
        }

        if (options.path) {
            __flags.push("--path");
        }

        if (options.plaintext) {
            __flags.push("--plaintext");
        }

        if (options.raw) {
            __flags.push("--raw");
        }

        if (options.secret) {
            __flags.push("--secret");
        }

        if (options.type != null) {
            __flags.push("--type", "" + options.type);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        __arguments.push("" + key);
        if (value != null) {
            __arguments.push("" + value);
        }
        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    configSetAll(options: PulumiConfigSetAllOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("config");
        __final.push("set-all");

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

        if (options.configFile != null) {
            __flags.push("--config-file", "" + options.configFile);
        }

        if (options.json != null) {
            __flags.push("--json", "" + options.json);
        }

        if (options.open) {
            __flags.push("--open");
        }

        if (options.showSecrets) {
            __flags.push("--show-secrets");
        }

        if (options.stack != null) {
            __flags.push("--stack", "" + options.stack);
        }

        if (options.path) {
            __flags.push("--path");
        }

        for (const __item of options.plaintext ?? []) {
            if (__item != null) {
                __flags.push("--plaintext", "" + __item);
            }
        }

        for (const __item of options.secret ?? []) {
            if (__item != null) {
                __flags.push("--secret", "" + __item);
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

    config(options: PulumiConfigOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("config");

        const __flags: string[] = [];

        __flags.push("--json");

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

        if (options.configFile != null) {
            __flags.push("--config-file", "" + options.configFile);
        }

        if (options.open) {
            __flags.push("--open");
        }

        if (options.showSecrets) {
            __flags.push("--show-secrets");
        }

        if (options.stack != null) {
            __flags.push("--stack", "" + options.stack);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    convert(options: PulumiConvertOptions, ...arg: string[]): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("convert");

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

        if (options.from != null) {
            __flags.push("--from", "" + options.from);
        }

        if (options.generateOnly) {
            __flags.push("--generate-only");
        }

        __flags.push("--language", "" + options.language);

        for (const __item of options.mappings ?? []) {
            if (__item != null) {
                __flags.push("--mappings", "" + __item);
            }
        }

        if (options.name != null) {
            __flags.push("--name", "" + options.name);
        }

        if (options.out != null) {
            __flags.push("--out", "" + options.out);
        }

        if (options.strict) {
            __flags.push("--strict");
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (arg != null) {
            for (const __item of arg ?? []) {
                __arguments.push("" + __item);
            }
        }
        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    import(options: PulumiImportOptions, ...arg: string[]): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("import");

        const __flags: string[] = [];

        __flags.push("--skip-preview");
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

        if (options.configFile != null) {
            __flags.push("--config-file", "" + options.configFile);
        }

        if (options.debug) {
            __flags.push("--debug");
        }

        if (options.diff) {
            __flags.push("--diff");
        }

        if (options.execAgent != null) {
            __flags.push("--exec-agent", "" + options.execAgent);
        }

        if (options.execKind != null) {
            __flags.push("--exec-kind", "" + options.execKind);
        }

        if (options.file != null) {
            __flags.push("--file", "" + options.file);
        }

        if (options.from != null) {
            __flags.push("--from", "" + options.from);
        }

        if (options.generateCode) {
            __flags.push("--generate-code");
        }

        if (options.generateResources != null) {
            __flags.push("--generate-resources", "" + options.generateResources);
        }

        if (options.json) {
            __flags.push("--json");
        }

        if (options.message != null) {
            __flags.push("--message", "" + options.message);
        }

        if (options.out != null) {
            __flags.push("--out", "" + options.out);
        }

        if (options.output != null) {
            __flags.push("--output", "" + options.output);
        }

        if (options.parallel != null) {
            __flags.push("--parallel", "" + options.parallel);
        }

        if (options.parent != null) {
            __flags.push("--parent", "" + options.parent);
        }

        if (options.previewOnly) {
            __flags.push("--preview-only");
        }

        for (const __item of options.properties ?? []) {
            if (__item != null) {
                __flags.push("--properties", "" + __item);
            }
        }

        if (options.protect) {
            __flags.push("--protect");
        }

        if (options.provider != null) {
            __flags.push("--provider", "" + options.provider);
        }

        if (options.skipPluginPreInstall) {
            __flags.push("--skip-plugin-pre-install");
        }

        if (options.stack != null) {
            __flags.push("--stack", "" + options.stack);
        }

        if (options.suppressOutputs) {
            __flags.push("--suppress-outputs");
        }

        if (options.suppressPermalink != null) {
            __flags.push("--suppress-permalink", "" + options.suppressPermalink);
        }

        if (options.suppressProgress) {
            __flags.push("--suppress-progress");
        }

        if (options.urns) {
            __flags.push("--urns");
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (arg != null) {
            for (const __item of arg ?? []) {
                __arguments.push("" + __item);
            }
        }
        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    install(options: PulumiInstallOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("install");

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

        if (options.noDependencies) {
            __flags.push("--no-dependencies");
        }

        if (options.noPlugins) {
            __flags.push("--no-plugins");
        }

        if (options.parallel != null) {
            __flags.push("--parallel", "" + options.parallel);
        }

        if (options.reinstall) {
            __flags.push("--reinstall");
        }

        if (options.useLanguageVersionTools) {
            __flags.push("--use-language-version-tools");
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

    refresh(options: PulumiRefreshOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("refresh");

        const __flags: string[] = [];

        __flags.push("--skip-preview");
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

        if (options.clearPendingCreates) {
            __flags.push("--clear-pending-creates");
        }

        if (options.client != null) {
            __flags.push("--client", "" + options.client);
        }

        for (const __item of options.config ?? []) {
            if (__item != null) {
                __flags.push("--config", "" + __item);
            }
        }

        if (options.configFile != null) {
            __flags.push("--config-file", "" + options.configFile);
        }

        if (options.configPath) {
            __flags.push("--config-path");
        }

        if (options.copilot) {
            __flags.push("--copilot");
        }

        if (options.debug) {
            __flags.push("--debug");
        }

        if (options.diff) {
            __flags.push("--diff");
        }

        for (const __item of options.exclude ?? []) {
            if (__item != null) {
                __flags.push("--exclude", "" + __item);
            }
        }

        if (options.excludeDependents) {
            __flags.push("--exclude-dependents");
        }

        if (options.execAgent != null) {
            __flags.push("--exec-agent", "" + options.execAgent);
        }

        if (options.execKind != null) {
            __flags.push("--exec-kind", "" + options.execKind);
        }

        if (options.expectNoChanges) {
            __flags.push("--expect-no-changes");
        }

        for (const __item of options.importPendingCreates ?? []) {
            if (__item != null) {
                __flags.push("--import-pending-creates", "" + __item);
            }
        }

        if (options.json) {
            __flags.push("--json");
        }

        if (options.message != null) {
            __flags.push("--message", "" + options.message);
        }

        if (options.neo) {
            __flags.push("--neo");
        }

        if (options.output != null) {
            __flags.push("--output", "" + options.output);
        }

        for (const __item of options.overrideEnv ?? []) {
            if (__item != null) {
                __flags.push("--override-env", "" + __item);
            }
        }

        if (options.parallel != null) {
            __flags.push("--parallel", "" + options.parallel);
        }

        if (options.previewOnly) {
            __flags.push("--preview-only");
        }

        if (options.runProgram) {
            __flags.push("--run-program");
        }

        if (options.showReplacementSteps) {
            __flags.push("--show-replacement-steps");
        }

        if (options.showSames) {
            __flags.push("--show-sames");
        }

        if (options.skipConfigValidation) {
            __flags.push("--skip-config-validation");
        }

        if (options.skipPendingCreates) {
            __flags.push("--skip-pending-creates");
        }

        if (options.skipPluginPreInstall) {
            __flags.push("--skip-plugin-pre-install");
        }

        if (options.stack != null) {
            __flags.push("--stack", "" + options.stack);
        }

        if (options.suppressOutputs) {
            __flags.push("--suppress-outputs");
        }

        if (options.suppressPermalink != null) {
            __flags.push("--suppress-permalink", "" + options.suppressPermalink);
        }

        if (options.suppressProgress) {
            __flags.push("--suppress-progress");
        }

        for (const __item of options.target ?? []) {
            if (__item != null) {
                __flags.push("--target", "" + __item);
            }
        }

        if (options.targetDependents) {
            __flags.push("--target-dependents");
        }

        if (options.urns) {
            __flags.push("--urns");
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    version(options: PulumiVersionOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
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

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    whoami(options: PulumiWhoamiOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("whoami");

        const __flags: string[] = [];

        __flags.push("--json");

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

        if (options.verbose) {
            __flags.push("--verbose");
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
    /** Export OpenTelemetry traces to the specified endpoint. Use file:// for local JSON files, grpc:// or https:// for remote collectors */
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

/** Options for the `pulumi config` command. */
export interface PulumiConfigOptions extends BaseOptions {
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
    /** Export OpenTelemetry traces to the specified endpoint. Use file:// for local JSON files, grpc:// or https:// for remote collectors */
    otelTraces?: string;
    /** Emit CPU and memory profiles and an execution trace to '[filename].[pid].{cpu,mem,trace}', respectively */
    profiling?: string;
    /** Emit tracing to the specified endpoint. Use the `file:` scheme to write tracing data to a local file */
    tracing?: string;
    /** Include the tracing header with the given contents. */
    tracingHeader?: string;
    /** Enable verbose logging (e.g., v=3); anything >3 is very verbose */
    verbose?: number;
    /** Use the configuration values in the specified file rather than detecting the file name */
    configFile?: string;
    /** Open and resolve any environments listed in the stack configuration. Defaults to true if --show-secrets is set, false otherwise */
    open?: boolean;
    /** Show secret values when listing config instead of displaying blinded values */
    showSecrets?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi config copy` command. */
export interface PulumiConfigCopyOptions extends BaseOptions {
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
    /** Export OpenTelemetry traces to the specified endpoint. Use file:// for local JSON files, grpc:// or https:// for remote collectors */
    otelTraces?: string;
    /** Emit CPU and memory profiles and an execution trace to '[filename].[pid].{cpu,mem,trace}', respectively */
    profiling?: string;
    /** Emit tracing to the specified endpoint. Use the `file:` scheme to write tracing data to a local file */
    tracing?: string;
    /** Include the tracing header with the given contents. */
    tracingHeader?: string;
    /** Enable verbose logging (e.g., v=3); anything >3 is very verbose */
    verbose?: number;
    /** Use the configuration values in the specified file rather than detecting the file name */
    configFile?: string;
    /** Emit output as JSON */
    json?: boolean;
    /** Open and resolve any environments listed in the stack configuration. Defaults to true if --show-secrets is set, false otherwise */
    open?: boolean;
    /** Show secret values when listing config instead of displaying blinded values */
    showSecrets?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** The name of the new stack to copy the config to */
    dest?: string;
    /** The key contains a path to a property in a map or list to set */
    path?: boolean;
}

/** Options for the `pulumi config env add` command. */
export interface PulumiConfigEnvAddOptions extends BaseOptions {
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
    /** Export OpenTelemetry traces to the specified endpoint. Use file:// for local JSON files, grpc:// or https:// for remote collectors */
    otelTraces?: string;
    /** Emit CPU and memory profiles and an execution trace to '[filename].[pid].{cpu,mem,trace}', respectively */
    profiling?: string;
    /** Emit tracing to the specified endpoint. Use the `file:` scheme to write tracing data to a local file */
    tracing?: string;
    /** Include the tracing header with the given contents. */
    tracingHeader?: string;
    /** Enable verbose logging (e.g., v=3); anything >3 is very verbose */
    verbose?: number;
    /** Use the configuration values in the specified file rather than detecting the file name */
    configFile?: string;
    /** Emit output as JSON */
    json?: boolean;
    /** Open and resolve any environments listed in the stack configuration. Defaults to true if --show-secrets is set, false otherwise */
    open?: boolean;
    /** Show secret values in plaintext instead of ciphertext */
    showSecrets?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi config env init` command. */
export interface PulumiConfigEnvInitOptions extends BaseOptions {
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
    /** Export OpenTelemetry traces to the specified endpoint. Use file:// for local JSON files, grpc:// or https:// for remote collectors */
    otelTraces?: string;
    /** Emit CPU and memory profiles and an execution trace to '[filename].[pid].{cpu,mem,trace}', respectively */
    profiling?: string;
    /** Emit tracing to the specified endpoint. Use the `file:` scheme to write tracing data to a local file */
    tracing?: string;
    /** Include the tracing header with the given contents. */
    tracingHeader?: string;
    /** Enable verbose logging (e.g., v=3); anything >3 is very verbose */
    verbose?: number;
    /** Use the configuration values in the specified file rather than detecting the file name */
    configFile?: string;
    /** Emit output as JSON */
    json?: boolean;
    /** Open and resolve any environments listed in the stack configuration. Defaults to true if --show-secrets is set, false otherwise */
    open?: boolean;
    /** Show secret values in plaintext instead of ciphertext */
    showSecrets?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** The name of the environment to create. Defaults to "<project name>/<stack name>" */
    env?: string;
    /** Do not remove configuration values from the stack after creating the environment */
    keepConfig?: boolean;
}

/** Options for the `pulumi config env list` command. */
export interface PulumiConfigEnvListOptions extends BaseOptions {
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
    /** Export OpenTelemetry traces to the specified endpoint. Use file:// for local JSON files, grpc:// or https:// for remote collectors */
    otelTraces?: string;
    /** Emit CPU and memory profiles and an execution trace to '[filename].[pid].{cpu,mem,trace}', respectively */
    profiling?: string;
    /** Emit tracing to the specified endpoint. Use the `file:` scheme to write tracing data to a local file */
    tracing?: string;
    /** Include the tracing header with the given contents. */
    tracingHeader?: string;
    /** Enable verbose logging (e.g., v=3); anything >3 is very verbose */
    verbose?: number;
    /** Use the configuration values in the specified file rather than detecting the file name */
    configFile?: string;
    /** Open and resolve any environments listed in the stack configuration. Defaults to true if --show-secrets is set, false otherwise */
    open?: boolean;
    /** Show secret values when listing config instead of displaying blinded values */
    showSecrets?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi config env remove` command. */
export interface PulumiConfigEnvRemoveOptions extends BaseOptions {
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
    /** Export OpenTelemetry traces to the specified endpoint. Use file:// for local JSON files, grpc:// or https:// for remote collectors */
    otelTraces?: string;
    /** Emit CPU and memory profiles and an execution trace to '[filename].[pid].{cpu,mem,trace}', respectively */
    profiling?: string;
    /** Emit tracing to the specified endpoint. Use the `file:` scheme to write tracing data to a local file */
    tracing?: string;
    /** Include the tracing header with the given contents. */
    tracingHeader?: string;
    /** Enable verbose logging (e.g., v=3); anything >3 is very verbose */
    verbose?: number;
    /** Use the configuration values in the specified file rather than detecting the file name */
    configFile?: string;
    /** Emit output as JSON */
    json?: boolean;
    /** Open and resolve any environments listed in the stack configuration. Defaults to true if --show-secrets is set, false otherwise */
    open?: boolean;
    /** Show secret values in plaintext instead of ciphertext */
    showSecrets?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi config get` command. */
export interface PulumiConfigGetOptions extends BaseOptions {
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
    /** Export OpenTelemetry traces to the specified endpoint. Use file:// for local JSON files, grpc:// or https:// for remote collectors */
    otelTraces?: string;
    /** Emit CPU and memory profiles and an execution trace to '[filename].[pid].{cpu,mem,trace}', respectively */
    profiling?: string;
    /** Emit tracing to the specified endpoint. Use the `file:` scheme to write tracing data to a local file */
    tracing?: string;
    /** Include the tracing header with the given contents. */
    tracingHeader?: string;
    /** Enable verbose logging (e.g., v=3); anything >3 is very verbose */
    verbose?: number;
    /** Use the configuration values in the specified file rather than detecting the file name */
    configFile?: string;
    /** Open and resolve any environments listed in the stack configuration */
    open?: boolean;
    /** Show secret values when listing config instead of displaying blinded values */
    showSecrets?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** The key contains a path to a property in a map or list to get */
    path?: boolean;
}

/** Options for the `pulumi config refresh` command. */
export interface PulumiConfigRefreshOptions extends BaseOptions {
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
    /** Export OpenTelemetry traces to the specified endpoint. Use file:// for local JSON files, grpc:// or https:// for remote collectors */
    otelTraces?: string;
    /** Emit CPU and memory profiles and an execution trace to '[filename].[pid].{cpu,mem,trace}', respectively */
    profiling?: string;
    /** Emit tracing to the specified endpoint. Use the `file:` scheme to write tracing data to a local file */
    tracing?: string;
    /** Include the tracing header with the given contents. */
    tracingHeader?: string;
    /** Enable verbose logging (e.g., v=3); anything >3 is very verbose */
    verbose?: number;
    /** Use the configuration values in the specified file rather than detecting the file name */
    configFile?: string;
    /** Emit output as JSON */
    json?: boolean;
    /** Open and resolve any environments listed in the stack configuration. Defaults to true if --show-secrets is set, false otherwise */
    open?: boolean;
    /** Show secret values when listing config instead of displaying blinded values */
    showSecrets?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi config remove` command. */
export interface PulumiConfigRemoveOptions extends BaseOptions {
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
    /** Export OpenTelemetry traces to the specified endpoint. Use file:// for local JSON files, grpc:// or https:// for remote collectors */
    otelTraces?: string;
    /** Emit CPU and memory profiles and an execution trace to '[filename].[pid].{cpu,mem,trace}', respectively */
    profiling?: string;
    /** Emit tracing to the specified endpoint. Use the `file:` scheme to write tracing data to a local file */
    tracing?: string;
    /** Include the tracing header with the given contents. */
    tracingHeader?: string;
    /** Enable verbose logging (e.g., v=3); anything >3 is very verbose */
    verbose?: number;
    /** Use the configuration values in the specified file rather than detecting the file name */
    configFile?: string;
    /** Emit output as JSON */
    json?: boolean;
    /** Open and resolve any environments listed in the stack configuration. Defaults to true if --show-secrets is set, false otherwise */
    open?: boolean;
    /** Show secret values when listing config instead of displaying blinded values */
    showSecrets?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** The key contains a path to a property in a map or list to remove */
    path?: boolean;
}

/** Options for the `pulumi config remove-all` command. */
export interface PulumiConfigRemoveAllOptions extends BaseOptions {
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
    /** Export OpenTelemetry traces to the specified endpoint. Use file:// for local JSON files, grpc:// or https:// for remote collectors */
    otelTraces?: string;
    /** Emit CPU and memory profiles and an execution trace to '[filename].[pid].{cpu,mem,trace}', respectively */
    profiling?: string;
    /** Emit tracing to the specified endpoint. Use the `file:` scheme to write tracing data to a local file */
    tracing?: string;
    /** Include the tracing header with the given contents. */
    tracingHeader?: string;
    /** Enable verbose logging (e.g., v=3); anything >3 is very verbose */
    verbose?: number;
    /** Use the configuration values in the specified file rather than detecting the file name */
    configFile?: string;
    /** Emit output as JSON */
    json?: boolean;
    /** Open and resolve any environments listed in the stack configuration. Defaults to true if --show-secrets is set, false otherwise */
    open?: boolean;
    /** Show secret values when listing config instead of displaying blinded values */
    showSecrets?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Parse the keys as paths in a map or list rather than raw strings */
    path?: boolean;
}

/** Options for the `pulumi config set` command. */
export interface PulumiConfigSetOptions extends BaseOptions {
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
    /** Export OpenTelemetry traces to the specified endpoint. Use file:// for local JSON files, grpc:// or https:// for remote collectors */
    otelTraces?: string;
    /** Emit CPU and memory profiles and an execution trace to '[filename].[pid].{cpu,mem,trace}', respectively */
    profiling?: string;
    /** Emit tracing to the specified endpoint. Use the `file:` scheme to write tracing data to a local file */
    tracing?: string;
    /** Include the tracing header with the given contents. */
    tracingHeader?: string;
    /** Enable verbose logging (e.g., v=3); anything >3 is very verbose */
    verbose?: number;
    /** Use the configuration values in the specified file rather than detecting the file name */
    configFile?: string;
    /** Emit output as JSON */
    json?: boolean;
    /** Open and resolve any environments listed in the stack configuration. Defaults to true if --show-secrets is set, false otherwise */
    open?: boolean;
    /** Show secret values when listing config instead of displaying blinded values */
    showSecrets?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** The key contains a path to a property in a map or list to set */
    path?: boolean;
    /** Save the value as plaintext (unencrypted) */
    plaintext?: boolean;
    /** When setting the value through stdin, do not trim trailing newlines from the value */
    raw?: boolean;
    /** Encrypt the value instead of storing it in plaintext */
    secret?: boolean;
    /** Save the value as the given type.  Allowed values are string, bool, int, and float */
    type?: string;
}

/** Options for the `pulumi config set-all` command. */
export interface PulumiConfigSetAllOptions extends BaseOptions {
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
    /** Export OpenTelemetry traces to the specified endpoint. Use file:// for local JSON files, grpc:// or https:// for remote collectors */
    otelTraces?: string;
    /** Emit CPU and memory profiles and an execution trace to '[filename].[pid].{cpu,mem,trace}', respectively */
    profiling?: string;
    /** Emit tracing to the specified endpoint. Use the `file:` scheme to write tracing data to a local file */
    tracing?: string;
    /** Include the tracing header with the given contents. */
    tracingHeader?: string;
    /** Enable verbose logging (e.g., v=3); anything >3 is very verbose */
    verbose?: number;
    /** Use the configuration values in the specified file rather than detecting the file name */
    configFile?: string;
    /** Read values from a JSON string in the format produced by 'pulumi config --json' */
    json?: string;
    /** Open and resolve any environments listed in the stack configuration. Defaults to true if --show-secrets is set, false otherwise */
    open?: boolean;
    /** Show secret values when listing config instead of displaying blinded values */
    showSecrets?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Parse the keys as paths in a map or list rather than raw strings */
    path?: boolean;
    /** Marks a value as plaintext (unencrypted) */
    plaintext?: string[];
    /** Marks a value as secret to be encrypted */
    secret?: string[];
}

/** Options for the `pulumi convert` command. */
export interface PulumiConvertOptions extends BaseOptions {
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
    /** Export OpenTelemetry traces to the specified endpoint. Use file:// for local JSON files, grpc:// or https:// for remote collectors */
    otelTraces?: string;
    /** Emit CPU and memory profiles and an execution trace to '[filename].[pid].{cpu,mem,trace}', respectively */
    profiling?: string;
    /** Emit tracing to the specified endpoint. Use the `file:` scheme to write tracing data to a local file */
    tracing?: string;
    /** Include the tracing header with the given contents. */
    tracingHeader?: string;
    /** Enable verbose logging (e.g., v=3); anything >3 is very verbose */
    verbose?: number;
    /** Which converter plugin to use to read the source program */
    from?: string;
    /** Generate the converted program(s) only; do not install dependencies */
    generateOnly?: boolean;
    /** Which language plugin to use to generate the Pulumi project */
    language: string;
    /** Any mapping files to use in the conversion */
    mappings?: string[];
    /** The name to use for the converted project; defaults to the directory of the source project */
    name?: string;
    /** The output directory to write the converted project to */
    out?: string;
    /** Fail the conversion on errors such as missing variables */
    strict?: boolean;
}

/** Options for the `pulumi import` command. */
export interface PulumiImportOptions extends BaseOptions {
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
    /** Export OpenTelemetry traces to the specified endpoint. Use file:// for local JSON files, grpc:// or https:// for remote collectors */
    otelTraces?: string;
    /** Emit CPU and memory profiles and an execution trace to '[filename].[pid].{cpu,mem,trace}', respectively */
    profiling?: string;
    /** Emit tracing to the specified endpoint. Use the `file:` scheme to write tracing data to a local file */
    tracing?: string;
    /** Include the tracing header with the given contents. */
    tracingHeader?: string;
    /** Enable verbose logging (e.g., v=3); anything >3 is very verbose */
    verbose?: number;
    /** Use the configuration values in the specified file rather than detecting the file name */
    configFile?: string;
    /** Print detailed debugging output during resource operations */
    debug?: boolean;
    /** Display operation as a rich diff showing the overall change */
    diff?: boolean;
    execAgent?: string;
    execKind?: string;
    /** The path to a JSON-encoded file containing a list of resources to import */
    file?: string;
    /** Invoke a converter to import the resources */
    from?: string;
    /** Generate resource declaration code for the imported resources */
    generateCode?: boolean;
    /** When used with --from, always write a JSON-encoded file containing a list of importable resources discovered by conversion to the specified path */
    generateResources?: string;
    /** Serialize the import diffs, operations, and overall output as JSON */
    json?: boolean;
    /** Optional message to associate with the update operation */
    message?: string;
    /** The path to the file that will contain the generated resource declarations */
    out?: string;
    /** Output format. Supported values are: default, json */
    output?: string;
    /** Allow P resource operations to run in parallel at once (1 for no parallelism). */
    parallel?: number;
    /** The name and URN of the parent resource in the format name=urn, where name is the variable name of the parent resource */
    parent?: string;
    /** Only show a preview of the import, but don't perform the import itself */
    previewOnly?: boolean;
    /** The property names to use for the import in the format name1,name2 */
    properties?: string[];
    /** Allow resources to be imported with protection from deletion enabled */
    protect?: boolean;
    /** The name and URN of the provider to use for the import in the format name=urn, where name is the variable name for the provider resource */
    provider?: string;
    /** Skip the up-front provider plugin install step; missing plugins are installed lazily by the engine */
    skipPluginPreInstall?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Suppress display of stack outputs (in case they contain sensitive values) */
    suppressOutputs?: boolean;
    /** Suppress display of the state permalink */
    suppressPermalink?: string;
    /** Suppress display of periodic progress dots */
    suppressProgress?: boolean;
    /** Display full URNs instead of short resource names */
    urns?: boolean;
}

/** Options for the `pulumi install` command. */
export interface PulumiInstallOptions extends BaseOptions {
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
    /** Export OpenTelemetry traces to the specified endpoint. Use file:// for local JSON files, grpc:// or https:// for remote collectors */
    otelTraces?: string;
    /** Emit CPU and memory profiles and an execution trace to '[filename].[pid].{cpu,mem,trace}', respectively */
    profiling?: string;
    /** Emit tracing to the specified endpoint. Use the `file:` scheme to write tracing data to a local file */
    tracing?: string;
    /** Include the tracing header with the given contents. */
    tracingHeader?: string;
    /** Enable verbose logging (e.g., v=3); anything >3 is very verbose */
    verbose?: number;
    /** Skip installing dependencies */
    noDependencies?: boolean;
    /** Skip installing plugins */
    noPlugins?: boolean;
    /** The max number of concurrent installs to perform. Parallelism of less than 1 implies unbounded parallelism */
    parallel?: number;
    /** Reinstall a plugin even if it already exists */
    reinstall?: boolean;
    /** Use language version tools to set up and install the language runtime */
    useLanguageVersionTools?: boolean;
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
    /** Export OpenTelemetry traces to the specified endpoint. Use file:// for local JSON files, grpc:// or https:// for remote collectors */
    otelTraces?: string;
    /** Emit CPU and memory profiles and an execution trace to '[filename].[pid].{cpu,mem,trace}', respectively */
    profiling?: string;
    /** Emit tracing to the specified endpoint. Use the `file:` scheme to write tracing data to a local file */
    tracing?: string;
    /** Include the tracing header with the given contents. */
    tracingHeader?: string;
    /** Enable verbose logging (e.g., v=3); anything >3 is very verbose */
    verbose?: number;
    /** Retired: use 'pulumi neo -p "prompt"' instead. */
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
    /** Retired: use 'pulumi neo -p "prompt"' instead. */
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
    /** Deprecated: template mode is now the only mode; this flag is a no-op */
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
    /** Export OpenTelemetry traces to the specified endpoint. Use file:// for local JSON files, grpc:// or https:// for remote collectors */
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
    /** Export OpenTelemetry traces to the specified endpoint. Use file:// for local JSON files, grpc:// or https:// for remote collectors */
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
    /** Export OpenTelemetry traces to the specified endpoint. Use file:// for local JSON files, grpc:// or https:// for remote collectors */
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
    /** Export OpenTelemetry traces to the specified endpoint. Use file:// for local JSON files, grpc:// or https:// for remote collectors */
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
    /** Export OpenTelemetry traces to the specified endpoint. Use file:// for local JSON files, grpc:// or https:// for remote collectors */
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

/** Options for the `pulumi refresh` command. */
export interface PulumiRefreshOptions extends BaseOptions {
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
    /** Export OpenTelemetry traces to the specified endpoint. Use file:// for local JSON files, grpc:// or https:// for remote collectors */
    otelTraces?: string;
    /** Emit CPU and memory profiles and an execution trace to '[filename].[pid].{cpu,mem,trace}', respectively */
    profiling?: string;
    /** Emit tracing to the specified endpoint. Use the `file:` scheme to write tracing data to a local file */
    tracing?: string;
    /** Include the tracing header with the given contents. */
    tracingHeader?: string;
    /** Enable verbose logging (e.g., v=3); anything >3 is very verbose */
    verbose?: number;
    /** Clear all pending creates, dropping them from the state */
    clearPendingCreates?: boolean;
    /** The address of an existing language runtime host to connect to */
    client?: string;
    /** Config to use during the refresh and save to the stack config file */
    config?: string[];
    /** Use the configuration values in the specified file rather than detecting the file name */
    configFile?: string;
    /** Config keys contain a path to a property in a map or list to set */
    configPath?: boolean;
    /** [DEPRECATED] Use --neo instead. Enable Pulumi Neo's assistance for improved CLI experience and insights (can also be set with PULUMI_COPILOT environment variable) */
    copilot?: boolean;
    /** Print detailed debugging output during resource operations */
    debug?: boolean;
    /** Display operation as a rich diff showing the overall change */
    diff?: boolean;
    /** Specify a resource URN to ignore. These resources will not be refreshed. Multiple resources can be specified using --exclude urn1 --exclude urn2. Wildcards (*, **) are also supported */
    exclude?: string[];
    /** Allows ignoring of dependent targets discovered but not specified in --exclude list */
    excludeDependents?: boolean;
    execAgent?: string;
    execKind?: string;
    /** Return an error if any changes occur during this refresh. This check happens after the refresh is applied */
    expectNoChanges?: boolean;
    /** A list of form [[URN ID]...] describing the provider IDs of pending creates */
    importPendingCreates?: string[];
    /** Serialize the refresh diffs, operations, and overall output as JSON */
    json?: boolean;
    /** Optional message to associate with the update operation */
    message?: string;
    /** Enable Pulumi Neo's assistance for improved CLI experience and insights (can also be set with PULUMI_NEO environment variable) */
    neo?: boolean;
    /** Output format. Supported values are: default, json */
    output?: string;
    /** [EXPERIMENTAL] Override an imported environment for this run only, as <env>=<replacement>; repeatable */
    overrideEnv?: string[];
    /** Allow P resource operations to run in parallel at once (1 for no parallelism). */
    parallel?: number;
    /** Only show a preview of the refresh, but don't perform the refresh itself */
    previewOnly?: boolean;
    /** Run the program to determine up-to-date state for providers to refresh resources */
    runProgram?: boolean;
    /** Show detailed resource replacement creates and deletes instead of a single step */
    showReplacementSteps?: boolean;
    /** Show resources that needn't be updated because they haven't changed, alongside those that do */
    showSames?: boolean;
    /** Skip validation of stack config values against the project config schema. Config validation is skipped automatically when --run-program is not set. */
    skipConfigValidation?: boolean;
    /** Skip importing pending creates in interactive mode */
    skipPendingCreates?: boolean;
    /** Skip the up-front provider plugin install step; missing plugins are installed lazily by the engine */
    skipPluginPreInstall?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Suppress display of stack outputs (in case they contain sensitive values) */
    suppressOutputs?: boolean;
    /** Suppress display of the state permalink */
    suppressPermalink?: string;
    /** Suppress display of periodic progress dots */
    suppressProgress?: boolean;
    /** Specify a single resource URN to refresh. Multiple resource can be specified using: --target urn1 --target urn2 */
    target?: string[];
    /** Allows updating of dependent targets discovered but not specified in --target list */
    targetDependents?: boolean;
    /** Display full URNs instead of short resource names */
    urns?: boolean;
}

/** Options for the `pulumi version` command. */
export interface PulumiVersionOptions extends BaseOptions {
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
    /** Export OpenTelemetry traces to the specified endpoint. Use file:// for local JSON files, grpc:// or https:// for remote collectors */
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

/** Options for the `pulumi whoami` command. */
export interface PulumiWhoamiOptions extends BaseOptions {
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
    /** Export OpenTelemetry traces to the specified endpoint. Use file:// for local JSON files, grpc:// or https:// for remote collectors */
    otelTraces?: string;
    /** Emit CPU and memory profiles and an execution trace to '[filename].[pid].{cpu,mem,trace}', respectively */
    profiling?: string;
    /** Emit tracing to the specified endpoint. Use the `file:` scheme to write tracing data to a local file */
    tracing?: string;
    /** Include the tracing header with the given contents. */
    tracingHeader?: string;
    /** Print detailed whoami information */
    verbose?: boolean;
    /** Output format. Supported values are: default and json */
    output?: string;
}
