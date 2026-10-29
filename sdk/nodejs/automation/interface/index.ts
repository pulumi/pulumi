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

    aboutEnv(options: PulumiAboutEnvOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("about");
        __final.push("env");

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

        if (options.json) {
            __flags.push("--json");
        }

        if (options.output != null) {
            __flags.push("--output", "" + options.output);
        }

        if (options.stack != null) {
            __flags.push("--stack", "" + options.stack);
        }

        if (options.transitive) {
            __flags.push("--transitive");
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    about(options: PulumiAboutOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("about");

        const __flags: string[] = [];

        if (options.color == null) {
            __flags.push("--color", "" + "never");
        }
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

        if (options.stack != null) {
            __flags.push("--stack", "" + options.stack);
        }

        if (options.transitive) {
            __flags.push("--transitive");
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
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

    envChangeRequestApply(options: PulumiEnvChangeRequestApplyOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("change-request");
        __final.push("apply");

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

        if (options.organization != null) {
            __flags.push("--organization", "" + options.organization);
        }

        if (options.dryRun) {
            __flags.push("--dry-run");
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

    envChangeRequestApprove(options: PulumiEnvChangeRequestApproveOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("change-request");
        __final.push("approve");

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

        if (options.organization != null) {
            __flags.push("--organization", "" + options.organization);
        }

        if (options.dryRun) {
            __flags.push("--dry-run");
        }

        if (options.message != null) {
            __flags.push("--message", "" + options.message);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envChangeRequestClose(options: PulumiEnvChangeRequestCloseOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("change-request");
        __final.push("close");

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

        if (options.message != null) {
            __flags.push("--message", "" + options.message);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envChangeRequestComment(options: PulumiEnvChangeRequestCommentOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("change-request");
        __final.push("comment");

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

        __flags.push("--message", "" + options.message);

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envChangeRequestDiff(options: PulumiEnvChangeRequestDiffOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("change-request");
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

        if (options.organization != null) {
            __flags.push("--organization", "" + options.organization);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envChangeRequestGet(options: PulumiEnvChangeRequestGetOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("change-request");
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

        if (options.organization != null) {
            __flags.push("--organization", "" + options.organization);
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

    envChangeRequestList(options: PulumiEnvChangeRequestListOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("change-request");
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

        if (options.organization != null) {
            __flags.push("--organization", "" + options.organization);
        }

        if (options.output != null) {
            __flags.push("--output", "" + options.output);
        }

        if (options.status != null) {
            __flags.push("--status", "" + options.status);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envChangeRequestLog(options: PulumiEnvChangeRequestLogOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("change-request");
        __final.push("log");

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

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envChangeRequestRebase(options: PulumiEnvChangeRequestRebaseOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("change-request");
        __final.push("rebase");

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

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envChangeRequestUnapprove(options: PulumiEnvChangeRequestUnapproveOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("change-request");
        __final.push("unapprove");

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

        __final.push(...__flags);

        const __arguments: string[] = [];

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

        if (options.reason != null) {
            __flags.push("--reason", "" + options.reason);
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

    envList(options: PulumiEnvListOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
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

        if (options.reason != null) {
            __flags.push("--reason", "" + options.reason);
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

        if (options.exportEnvVars) {
            __flags.push("--export-env-vars");
        }

        if (options.path != null) {
            __flags.push("--path", "" + options.path);
        }

        for (const __item of options.policyArn ?? []) {
            if (__item != null) {
                __flags.push("--policy-arn", "" + __item);
            }
        }

        if (options.reason != null) {
            __flags.push("--reason", "" + options.reason);
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

        if (options.exportEnvVars) {
            __flags.push("--export-env-vars");
        }

        if (options.path != null) {
            __flags.push("--path", "" + options.path);
        }

        if (options.reason != null) {
            __flags.push("--reason", "" + options.reason);
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

        if (options.exportEnvVars) {
            __flags.push("--export-env-vars");
        }

        if (options.path != null) {
            __flags.push("--path", "" + options.path);
        }

        if (options.reason != null) {
            __flags.push("--reason", "" + options.reason);
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

        if (options.exportEnvVars) {
            __flags.push("--export-env-vars");
        }

        if (options.path != null) {
            __flags.push("--path", "" + options.path);
        }

        if (options.reason != null) {
            __flags.push("--reason", "" + options.reason);
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

        if (options.exportEnvVars) {
            __flags.push("--export-env-vars");
        }

        if (options.path != null) {
            __flags.push("--path", "" + options.path);
        }

        __flags.push("--provider-id", "" + options.providerId);

        if (options.reason != null) {
            __flags.push("--reason", "" + options.reason);
        }

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

        if (options.exportEnvVars) {
            __flags.push("--export-env-vars");
        }

        if (options.path != null) {
            __flags.push("--path", "" + options.path);
        }

        if (options.reason != null) {
            __flags.push("--reason", "" + options.reason);
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

    envRemove(options: PulumiEnvRemoveOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
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

        if (options.reason != null) {
            __flags.push("--reason", "" + options.reason);
        }

        if (options.secret) {
            __flags.push("--secret");
        }

        if (options.string_) {
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

    envSetupAws(options: PulumiEnvSetupAwsOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("setup");
        __final.push("aws");

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

        for (const __item of options.account ?? []) {
            if (__item != null) {
                __flags.push("--account", "" + __item);
            }
        }

        if (options.duration != null) {
            __flags.push("--duration", "" + options.duration);
        }

        if (options.org != null) {
            __flags.push("--org", "" + options.org);
        }

        if (options.organization != null) {
            __flags.push("--organization", "" + options.organization);
        }

        if (options.policy != null) {
            __flags.push("--policy", "" + options.policy);
        }

        if (options.project != null) {
            __flags.push("--project", "" + options.project);
        }

        if (options.sessionName != null) {
            __flags.push("--session-name", "" + options.sessionName);
        }

        if (options.sso) {
            __flags.push("--sso");
        }

        if (options.ssoRegion != null) {
            __flags.push("--sso-region", "" + options.ssoRegion);
        }

        if (options.ssoRole != null) {
            __flags.push("--sso-role", "" + options.ssoRole);
        }

        if (options.ssoStartUrl != null) {
            __flags.push("--sso-start-url", "" + options.ssoStartUrl);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envSetupAzure(options: PulumiEnvSetupAzureOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("setup");
        __final.push("azure");

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

        if (options.browser) {
            __flags.push("--browser");
        }

        if (options.org != null) {
            __flags.push("--org", "" + options.org);
        }

        if (options.organization != null) {
            __flags.push("--organization", "" + options.organization);
        }

        if (options.policy != null) {
            __flags.push("--policy", "" + options.policy);
        }

        if (options.project != null) {
            __flags.push("--project", "" + options.project);
        }

        for (const __item of options.subscription ?? []) {
            if (__item != null) {
                __flags.push("--subscription", "" + __item);
            }
        }

        if (options.tenant != null) {
            __flags.push("--tenant", "" + options.tenant);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envSetupGcp(options: PulumiEnvSetupGcpOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("setup");
        __final.push("gcp");

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

        if (options.org != null) {
            __flags.push("--org", "" + options.org);
        }

        if (options.organization != null) {
            __flags.push("--organization", "" + options.organization);
        }

        if (options.policy != null) {
            __flags.push("--policy", "" + options.policy);
        }

        if (options.project != null) {
            __flags.push("--project", "" + options.project);
        }

        for (const __item of options.projectId ?? []) {
            if (__item != null) {
                __flags.push("--project-id", "" + __item);
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

    envTagList(options: PulumiEnvTagListOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("tag");
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

    envTagMove(options: PulumiEnvTagMoveOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("tag");
        __final.push("move");

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

    envTagRemove(options: PulumiEnvTagRemoveOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("tag");
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

        if (options.reason != null) {
            __flags.push("--reason", "" + options.reason);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    envVersionTagList(options: PulumiEnvVersionTagListOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("version");
        __final.push("tag");
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

    envVersionTagRemove(options: PulumiEnvVersionTagRemoveOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("version");
        __final.push("tag");
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

    envWebhookRemove(options: PulumiEnvWebhookRemoveOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("env");
        __final.push("webhook");
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

    logsDecrypt(options: PulumiLogsDecryptOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("logs");
        __final.push("decrypt");

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

        if (options.stack != null) {
            __flags.push("--stack", "" + options.stack);
        }

        if (options.latest) {
            __flags.push("--latest");
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    logsList(options: PulumiLogsListOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("logs");
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

        if (options.stack != null) {
            __flags.push("--stack", "" + options.stack);
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

    logsRemove(options: PulumiLogsRemoveOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("logs");
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

        if (options.stack != null) {
            __flags.push("--stack", "" + options.stack);
        }

        if (options.all) {
            __flags.push("--all");
        }

        if (options.before != null) {
            __flags.push("--before", "" + options.before);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    logsShare(options: PulumiLogsShareOptions, filename?: string): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("logs");
        __final.push("share");

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

        if (options.stack != null) {
            __flags.push("--stack", "" + options.stack);
        }

        if (options.includeSecrets) {
            __flags.push("--include-secrets");
        }

        if (options.latest) {
            __flags.push("--latest");
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (filename != null) {
            __arguments.push("" + filename);
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

    orgAuditLogExport(options: PulumiOrgAuditLogExportOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("org");
        __final.push("audit-log");
        __final.push("export");

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

        if (options.count != null) {
            __flags.push("--count", "" + options.count);
        }

        if (options.endTime != null) {
            __flags.push("--end-time", "" + options.endTime);
        }

        if (options.eventType != null) {
            __flags.push("--event-type", "" + options.eventType);
        }

        if (options.format != null) {
            __flags.push("--format", "" + options.format);
        }

        if (options.org != null) {
            __flags.push("--org", "" + options.org);
        }

        if (options.output != null) {
            __flags.push("--output", "" + options.output);
        }

        if (options.startTime != null) {
            __flags.push("--start-time", "" + options.startTime);
        }

        if (options.user != null) {
            __flags.push("--user", "" + options.user);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    orgAuditLogList(options: PulumiOrgAuditLogListOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("org");
        __final.push("audit-log");
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

        if (options.all) {
            __flags.push("--all");
        }

        if (options.count != null) {
            __flags.push("--count", "" + options.count);
        }

        if (options.endTime != null) {
            __flags.push("--end-time", "" + options.endTime);
        }

        if (options.eventType != null) {
            __flags.push("--event-type", "" + options.eventType);
        }

        if (options.org != null) {
            __flags.push("--org", "" + options.org);
        }

        if (options.output != null) {
            __flags.push("--output", "" + options.output);
        }

        if (options.startTime != null) {
            __flags.push("--start-time", "" + options.startTime);
        }

        if (options.user != null) {
            __flags.push("--user", "" + options.user);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    orgAuditLog(options: PulumiOrgAuditLogOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("org");
        __final.push("audit-log");

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

    orgMemberEdit(options: PulumiOrgMemberEditOptions, userLogin: string): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("org");
        __final.push("member");
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

        if (options.fgaRoleId != null) {
            __flags.push("--fga-role-id", "" + options.fgaRoleId);
        }

        if (options.fgaRoleName != null) {
            __flags.push("--fga-role-name", "" + options.fgaRoleName);
        }

        if (options.org != null) {
            __flags.push("--org", "" + options.org);
        }

        if (options.output != null) {
            __flags.push("--output", "" + options.output);
        }

        if (options.role != null) {
            __flags.push("--role", "" + options.role);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        __arguments.push("" + userLogin);
        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    orgMemberList(options: PulumiOrgMemberListOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("org");
        __final.push("member");
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

        if (options.all) {
            __flags.push("--all");
        }

        if (options.count != null) {
            __flags.push("--count", "" + options.count);
        }

        if (options.org != null) {
            __flags.push("--org", "" + options.org);
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

    orgMemberRemove(options: PulumiOrgMemberRemoveOptions, userLogin: string): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("org");
        __final.push("member");
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

        if (options.org != null) {
            __flags.push("--org", "" + options.org);
        }

        if (options.output != null) {
            __flags.push("--output", "" + options.output);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        __arguments.push("" + userLogin);
        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    orgMember(options: PulumiOrgMemberOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("org");
        __final.push("member");

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

    orgUsageGet(options: PulumiOrgUsageGetOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("org");
        __final.push("usage");
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

        if (options.granularity != null) {
            __flags.push("--granularity", "" + options.granularity);
        }

        if (options.lookbackDays != null) {
            __flags.push("--lookback-days", "" + options.lookbackDays);
        }

        if (options.lookbackStart != null) {
            __flags.push("--lookback-start", "" + options.lookbackStart);
        }

        if (options.org != null) {
            __flags.push("--org", "" + options.org);
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

    orgUsage(options: PulumiOrgUsageOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("org");
        __final.push("usage");

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

    stateEdit(options: PulumiStateEditOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("state");
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

    stateGet(options: PulumiStateGetOptions, resource?: string): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("state");
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

        if (options.output != null) {
            __flags.push("--output", "" + options.output);
        }

        if (options.showSecrets) {
            __flags.push("--show-secrets");
        }

        if (options.stack != null) {
            __flags.push("--stack", "" + options.stack);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (resource != null) {
            __arguments.push("" + resource);
        }
        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    stateMove(options: PulumiStateMoveOptions, ...urn: string[]): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("state");
        __final.push("move");

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

        if (options.dest != null) {
            __flags.push("--dest", "" + options.dest);
        }

        if (options.includeParents) {
            __flags.push("--include-parents");
        }

        if (options.source != null) {
            __flags.push("--source", "" + options.source);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        for (const __item of urn ?? []) {
            __arguments.push("" + __item);
        }
        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    statePromote(options: PulumiStatePromoteOptions, snippetName: string): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("state");
        __final.push("promote");

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

        __arguments.push("" + snippetName);
        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    stateProtect(options: PulumiStateProtectOptions, ...resourceUrn: string[]): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("state");
        __final.push("protect");

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

        if (options.all) {
            __flags.push("--all");
        }

        if (options.stack != null) {
            __flags.push("--stack", "" + options.stack);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (resourceUrn != null) {
            for (const __item of resourceUrn ?? []) {
                __arguments.push("" + __item);
            }
        }
        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    stateRemove(options: PulumiStateRemoveOptions, ...resourceUrn: string[]): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("state");
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

        if (options.all) {
            __flags.push("--all");
        }

        if (options.force) {
            __flags.push("--force");
        }

        if (options.stack != null) {
            __flags.push("--stack", "" + options.stack);
        }

        if (options.targetDependents) {
            __flags.push("--target-dependents");
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (resourceUrn != null) {
            for (const __item of resourceUrn ?? []) {
                __arguments.push("" + __item);
            }
        }
        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    stateRename(options: PulumiStateRenameOptions, resourceUrn?: string, newName?: string): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("state");
        __final.push("rename");

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

        if (resourceUrn != null) {
            __arguments.push("" + resourceUrn);
        }
        if (newName != null) {
            __arguments.push("" + newName);
        }
        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    stateRepair(options: PulumiStateRepairOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("state");
        __final.push("repair");

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

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    stateTaint(options: PulumiStateTaintOptions, ...resourceUrn: string[]): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("state");
        __final.push("taint");

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

        if (resourceUrn != null) {
            for (const __item of resourceUrn ?? []) {
                __arguments.push("" + __item);
            }
        }
        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    stateUnprotect(options: PulumiStateUnprotectOptions, ...resourceUrn: string[]): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("state");
        __final.push("unprotect");

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

        if (options.all) {
            __flags.push("--all");
        }

        if (options.stack != null) {
            __flags.push("--stack", "" + options.stack);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (resourceUrn != null) {
            for (const __item of resourceUrn ?? []) {
                __arguments.push("" + __item);
            }
        }
        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    stateUntaint(options: PulumiStateUntaintOptions, ...resourceUrn: string[]): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("state");
        __final.push("untaint");

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

        if (options.all) {
            __flags.push("--all");
        }

        if (options.stack != null) {
            __flags.push("--stack", "" + options.stack);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (resourceUrn != null) {
            for (const __item of resourceUrn ?? []) {
                __arguments.push("" + __item);
            }
        }
        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    stateUpgrade(options: PulumiStateUpgradeOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("state");
        __final.push("upgrade");

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

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    up(options: PulumiUpOptions, templateOrUrl?: string): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("up");

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

        for (const __item of options.attachDebugger ?? []) {
            if (__item != null) {
                __flags.push("--attach-debugger", "" + __item);
            }
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

        if (options.continueOnError) {
            __flags.push("--continue-on-error");
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

        if (options.ignoreProtect) {
            __flags.push("--ignore-protect");
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

        if (options.neoTaskOnFailure) {
            __flags.push("--neo-task-on-failure");
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

        if (options.plan != null) {
            __flags.push("--plan", "" + options.plan);
        }

        for (const __item of options.policyPack ?? []) {
            if (__item != null) {
                __flags.push("--policy-pack", "" + __item);
            }
        }

        for (const __item of options.policyPackConfig ?? []) {
            if (__item != null) {
                __flags.push("--policy-pack-config", "" + __item);
            }
        }

        if (options.refresh != null) {
            __flags.push("--refresh", "" + options.refresh);
        }

        for (const __item of options.replace ?? []) {
            if (__item != null) {
                __flags.push("--replace", "" + __item);
            }
        }

        if (options.runProgram) {
            __flags.push("--run-program");
        }

        if (options.secretsProvider != null) {
            __flags.push("--secrets-provider", "" + options.secretsProvider);
        }

        if (options.showConfig) {
            __flags.push("--show-config");
        }

        if (options.showFullOutput) {
            __flags.push("--show-full-output");
        }

        if (options.showPolicyExceptions) {
            __flags.push("--show-policy-exceptions");
        }

        if (options.showPolicyRemediations) {
            __flags.push("--show-policy-remediations");
        }

        if (options.showReads) {
            __flags.push("--show-reads");
        }

        if (options.showReplacementSteps) {
            __flags.push("--show-replacement-steps");
        }

        if (options.showSames) {
            __flags.push("--show-sames");
        }

        if (options.showSecrets) {
            __flags.push("--show-secrets");
        }

        if (options.skipConfigValidation) {
            __flags.push("--skip-config-validation");
        }

        if (options.skipPluginPreInstall) {
            __flags.push("--skip-plugin-pre-install");
        }

        if (options.stack != null) {
            __flags.push("--stack", "" + options.stack);
        }

        if (options.strict) {
            __flags.push("--strict");
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

        for (const __item of options.targetReplace ?? []) {
            if (__item != null) {
                __flags.push("--target-replace", "" + __item);
            }
        }

        if (options.urns) {
            __flags.push("--urns");
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

/** Options for the `pulumi about` command. */
export interface PulumiAboutOptions extends BaseOptions {
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
    /** The name of the stack to get info on. Defaults to the current stack */
    stack?: string;
    /** Include transitive dependencies */
    transitive?: boolean;
}

/** Options for the `pulumi about env` command. */
export interface PulumiAboutEnvOptions extends BaseOptions {
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
    /** Emit output as JSON (alias for --output=json) */
    json?: boolean;
    /** Output format. Supported values are: default and json */
    output?: string;
    /** The name of the stack to get info on. Defaults to the current stack */
    stack?: string;
    /** Include transitive dependencies */
    transitive?: boolean;
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

/** Options for the `pulumi env change-request apply` command. */
export interface PulumiEnvChangeRequestApplyOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** the organization that owns the change request (defaults to your default organization) */
    organization?: string;
    /** show the proposed change without applying it */
    dryRun?: boolean;
    /** output format: "text" (default) or "json" */
    output?: string;
}

/** Options for the `pulumi env change-request approve` command. */
export interface PulumiEnvChangeRequestApproveOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** the organization that owns the change request (defaults to your default organization) */
    organization?: string;
    /** show the proposed change without approving it */
    dryRun?: boolean;
    /** a comment to add with the approval */
    message?: string;
}

/** Options for the `pulumi env change-request close` command. */
export interface PulumiEnvChangeRequestCloseOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** the organization that owns the change request (defaults to your default organization) */
    organization?: string;
    /** a comment explaining why the change request was closed */
    message?: string;
}

/** Options for the `pulumi env change-request comment` command. */
export interface PulumiEnvChangeRequestCommentOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** the organization that owns the change request (defaults to your default organization) */
    organization?: string;
    /** the comment */
    message: string;
}

/** Options for the `pulumi env change-request diff` command. */
export interface PulumiEnvChangeRequestDiffOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** the organization that owns the change request (defaults to your default organization) */
    organization?: string;
}

/** Options for the `pulumi env change-request get` command. */
export interface PulumiEnvChangeRequestGetOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** the organization that owns the change request (defaults to your default organization) */
    organization?: string;
    /** output format: "text" (default) or "json" */
    output?: string;
}

/** Options for the `pulumi env change-request list` command. */
export interface PulumiEnvChangeRequestListOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** the organization that owns the change request (defaults to your default organization) */
    organization?: string;
    /** output format: "text" (default) or "json" */
    output?: string;
    /** only list change requests with this status: open (draft, pending, or ready), applied, closed, or all */
    status?: string;
}

/** Options for the `pulumi env change-request log` command. */
export interface PulumiEnvChangeRequestLogOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** the organization that owns the change request (defaults to your default organization) */
    organization?: string;
    /** output format: "text" (default) or "json" */
    output?: string;
}

/** Options for the `pulumi env change-request rebase` command. */
export interface PulumiEnvChangeRequestRebaseOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** the organization that owns the change request (defaults to your default organization) */
    organization?: string;
    /** output format: "text" (default) or "json" */
    output?: string;
}

/** Options for the `pulumi env change-request unapprove` command. */
export interface PulumiEnvChangeRequestUnapproveOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** the organization that owns the change request (defaults to your default organization) */
    organization?: string;
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
    /** The name of the environment to operate on. */
    env?: string;
    /** set flag without a value (--draft) to create a draft rather than saving changes directly. --draft=<change-request-id> to update an existing change request. */
    draft?: string;
    /** the command to use to edit the environment definition */
    editor?: string;
    /** the file that contains the updated environment, if any. Pass `-` to read from standard input. */
    file?: string;
    /** the description for the change request created by --draft */
    reason?: string;
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
    /** The name of the environment to operate on. */
    env?: string;
    /** the file to use to initialize the environment, if any. Pass `-` to read from standard input. */
    file?: string;
}

/** Options for the `pulumi env list` command. */
export interface PulumiEnvListOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** duration of access in seconds */
    accessDurationSeconds?: string;
    /** expiration time for the grant in seconds */
    grantExpirationSeconds?: string;
    /** output format: "text" (default) or "json" */
    output?: string;
    /** an optional reason explaining why the environment is being opened, shown to approvers */
    reason?: string;
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
    /** The name of the environment to operate on. */
    env?: string;
    /** create the environment if it does not already exist */
    create?: boolean;
    /** set flag without a value (--draft) to create a draft rather than saving changes directly. --draft=<change-request-id> to update an existing change request. */
    draft?: string;
    /** optional session duration, e.g. 1h */
    duration?: string;
    /** also set the AWS SDK environment variables (AWS_ACCESS_KEY_ID, etc.) referencing the login outputs */
    exportEnvVars?: boolean;
    /** property path under `values` where the provider block is written */
    path?: string;
    /** AWS managed-policy ARN to attach to the role session (repeatable) */
    policyArn?: string[];
    /** the description for the change request created by --draft */
    reason?: string;
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
    /** The name of the environment to operate on. */
    env?: string;
    /** create the environment if it does not already exist */
    create?: boolean;
    /** set flag without a value (--draft) to create a draft rather than saving changes directly. --draft=<change-request-id> to update an existing change request. */
    draft?: string;
    /** also set the AWS SDK environment variables (AWS_ACCESS_KEY_ID, etc.) referencing the login outputs */
    exportEnvVars?: boolean;
    /** property path under `values` where the provider block is written */
    path?: string;
    /** the description for the change request created by --draft */
    reason?: string;
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
    /** The name of the environment to operate on. */
    env?: string;
    /** create the environment if it does not already exist */
    create?: boolean;
    /** set flag without a value (--draft) to create a draft rather than saving changes directly. --draft=<change-request-id> to update an existing change request. */
    draft?: string;
    /** also set the Azure environment variables (ARM_CLIENT_ID, etc.) referencing the login outputs */
    exportEnvVars?: boolean;
    /** property path under `values` where the provider block is written */
    path?: string;
    /** the description for the change request created by --draft */
    reason?: string;
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
    /** The name of the environment to operate on. */
    env?: string;
    /** create the environment if it does not already exist */
    create?: boolean;
    /** set flag without a value (--draft) to create a draft rather than saving changes directly. --draft=<change-request-id> to update an existing change request. */
    draft?: string;
    /** also set the Azure environment variables (ARM_CLIENT_ID, etc.) referencing the login outputs */
    exportEnvVars?: boolean;
    /** property path under `values` where the provider block is written */
    path?: string;
    /** the description for the change request created by --draft */
    reason?: string;
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
    /** The name of the environment to operate on. */
    env?: string;
    /** create the environment if it does not already exist */
    create?: boolean;
    /** set flag without a value (--draft) to create a draft rather than saving changes directly. --draft=<change-request-id> to update an existing change request. */
    draft?: string;
    /** also set the Google SDK environment variables (GOOGLE_PROJECT, etc.) referencing the login outputs */
    exportEnvVars?: boolean;
    /** property path under `values` where the provider block is written */
    path?: string;
    /** GCP workload identity pool provider ID (required) */
    providerId: string;
    /** the description for the change request created by --draft */
    reason?: string;
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
    /** The name of the environment to operate on. */
    env?: string;
    /** create the environment if it does not already exist */
    create?: boolean;
    /** set flag without a value (--draft) to create a draft rather than saving changes directly. --draft=<change-request-id> to update an existing change request. */
    draft?: string;
    /** also set the Google SDK environment variables (GOOGLE_PROJECT, etc.) referencing the login outputs */
    exportEnvVars?: boolean;
    /** property path under `values` where the provider block is written */
    path?: string;
    /** the description for the change request created by --draft */
    reason?: string;
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

/** Options for the `pulumi env remove` command. */
export interface PulumiEnvRemoveOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** set flag without a value (--draft) to create a draft rather than saving changes directly. --draft=<change-request-id> to update an existing change request. */
    draft?: string;
    /** If set, the value is read from the specified file. Pass `-` to read from standard input. */
    file?: string;
    /** true to leave the value in plaintext */
    plaintext?: boolean;
    /** the description for the change request created by --draft */
    reason?: string;
    /** true to mark the value as secret */
    secret?: boolean;
    /** true to treat the value as a string rather than attempting to parse it as YAML */
    string_?: boolean;
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
    /** The name of the environment to operate on. */
    env?: string;
}

/** Options for the `pulumi env setup aws` command. */
export interface PulumiEnvSetupAwsOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** an AWS `account` to set up (repeatable; prompted for when omitted) */
    account?: string[];
    /** the session duration for the assumed role */
    duration?: string;
    /** the Pulumi organization to configure OIDC for */
    org?: string;
    /** the Pulumi organization to configure OIDC for */
    organization?: string;
    /** the policy attached to the OIDC role: AdministratorAccess (required for Deployments), ReadOnlyAccess (required for Discovery), or any other policy ARN; prompted for when omitted */
    policy?: string;
    /** the ESC project that per-account environments are created in */
    project?: string;
    /** the AWS session name recorded for the assumed role */
    sessionName?: string;
    /** force AWS SSO browser sign-in instead of using existing credentials, so several accounts can be configured at once (the SSO start URL and region are inferred from your AWS config) */
    sso?: boolean;
    /** the region the AWS SSO instance is hosted in (inferred from your AWS config when omitted) */
    ssoRegion?: string;
    /** the SSO role to assume when setting up each account (prompted for when ambiguous) */
    ssoRole?: string;
    /** the AWS SSO start URL; forces the browser sign-in (inferred from your AWS config when omitted) */
    ssoStartUrl?: string;
}

/** Options for the `pulumi env setup azure` command. */
export interface PulumiEnvSetupAzureOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** force browser sign-in instead of using existing credentials */
    browser?: boolean;
    /** the Pulumi organization to configure OIDC for */
    org?: string;
    /** the Pulumi organization to configure OIDC for */
    organization?: string;
    /** the role assigned per subscription: Contributor (required for Deployments), Reader (required for Discovery), or any other role definition ID; prompted for when omitted */
    policy?: string;
    /** the ESC project that per-subscription environments are created in */
    project?: string;
    /** an Azure `subscription` to set up (repeatable; prompted for when omitted) */
    subscription?: string[];
    /** the Azure tenant to configure, which only its own subscriptions are visible through (prompted for when omitted) */
    tenant?: string;
}

/** Options for the `pulumi env setup gcp` command. */
export interface PulumiEnvSetupGcpOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** the Pulumi organization to configure OIDC for */
    org?: string;
    /** the Pulumi organization to configure OIDC for */
    organization?: string;
    /** the role granted to the service account: roles/editor (required for Deployments), roles/viewer (required for Discovery), or any other role; prompted for when omitted */
    policy?: string;
    /** the ESC project that per-project environments are created in */
    project?: string;
    /** a GCP `project` to set up (repeatable; prompted for when omitted) */
    projectId?: string[];
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
    /** The name of the environment to operate on. */
    env?: string;
    /** display times in UTC */
    utc?: boolean;
    /** output format: "text" (default) or "json" */
    output?: string;
}

/** Options for the `pulumi env tag list` command. */
export interface PulumiEnvTagListOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** display times in UTC */
    utc?: boolean;
    /** output format: "text" (default) or "json" */
    output?: string;
    /** the command to use to page through the environment's version tags */
    pager?: string;
}

/** Options for the `pulumi env tag move` command. */
export interface PulumiEnvTagMoveOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** display times in UTC */
    utc?: boolean;
}

/** Options for the `pulumi env tag remove` command. */
export interface PulumiEnvTagRemoveOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** display times in UTC */
    utc?: boolean;
    /** set flag without a value (--draft) to create a draft rather than saving changes directly. --draft=<change-request-id> to update an existing change request. */
    draft?: string;
    /** the description for the change request created by --draft */
    reason?: string;
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
    /** The name of the environment to operate on. */
    env?: string;
    /** display times in UTC */
    utc?: boolean;
}

/** Options for the `pulumi env version tag list` command. */
export interface PulumiEnvVersionTagListOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
    /** display times in UTC */
    utc?: boolean;
    /** output format: "text" (default) or "json" */
    output?: string;
    /** the command to use to page through the environment's version tags */
    pager?: string;
}

/** Options for the `pulumi env version tag remove` command. */
export interface PulumiEnvVersionTagRemoveOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
}

/** Options for the `pulumi env webhook remove` command. */
export interface PulumiEnvWebhookRemoveOptions extends BaseOptions {
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
    /** The name of the environment to operate on. */
    env?: string;
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

/** Options for the `pulumi logs decrypt` command. */
export interface PulumiLogsDecryptOptions extends BaseOptions {
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
    /** Decrypt the most recent log file without prompting */
    latest?: boolean;
}

/** Options for the `pulumi logs list` command. */
export interface PulumiLogsListOptions extends BaseOptions {
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
    /** Output format. Supported values are: default and json */
    output?: string;
}

/** Options for the `pulumi logs remove` command. */
export interface PulumiLogsRemoveOptions extends BaseOptions {
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
    /** Remove all log files */
    all?: boolean;
    /** Remove logs created before this date or duration (e.g. '24h', '2026-01-01') */
    before?: string;
}

/** Options for the `pulumi logs share` command. */
export interface PulumiLogsShareOptions extends BaseOptions {
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
    /** Include secret values in the shared log (by default secrets are redacted) */
    includeSecrets?: boolean;
    /** Share the most recent log file without prompting */
    latest?: boolean;
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

/** Options for the `pulumi org audit-log` command. */
export interface PulumiOrgAuditLogOptions extends BaseOptions {
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

/** Options for the `pulumi org audit-log export` command. */
export interface PulumiOrgAuditLogExportOptions extends BaseOptions {
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
    /** Truncate the exported response to the given number of bytes (0 returns the full response) */
    count?: number;
    /** The upper bound of the time range, as a Unix timestamp */
    endTime?: string;
    /** Filter by event type */
    eventType?: string;
    /** The export format. One of: csv, cef */
    format?: string;
    /** The organization to export audit logs for */
    org?: string;
    /** Output format. Supported values are: default and json */
    output?: string;
    /** The lower bound of the time range, as a Unix timestamp */
    startTime?: string;
    /** Filter by user login */
    user?: string;
}

/** Options for the `pulumi org audit-log list` command. */
export interface PulumiOrgAuditLogListOptions extends BaseOptions {
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
    /** Return all matching events; mutually exclusive with --count */
    all?: boolean;
    /** Maximum number of events to return. Defaults to the size of the first page; larger values auto-paginate */
    count?: number;
    /** The upper bound of the time range, as a Unix timestamp */
    endTime?: string;
    /** Filter by event type */
    eventType?: string;
    /** The organization to list audit logs for */
    org?: string;
    /** Output format. Supported values are: default and json */
    output?: string;
    /** The lower bound of the time range, as a Unix timestamp */
    startTime?: string;
    /** Filter by user login */
    user?: string;
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

/** Options for the `pulumi org member` command. */
export interface PulumiOrgMemberOptions extends BaseOptions {
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

/** Options for the `pulumi org member edit` command. */
export interface PulumiOrgMemberEditOptions extends BaseOptions {
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
    /** The custom role to assign (by ID) */
    fgaRoleId?: string;
    /** The custom role to assign (by name; resolved to ID automatically) */
    fgaRoleName?: string;
    /** The organization that owns the member */
    org?: string;
    /** Output format. Supported values are: default and json */
    output?: string;
    /** The built-in role to assign: member, admin, or billing-manager */
    role?: string;
}

/** Options for the `pulumi org member list` command. */
export interface PulumiOrgMemberListOptions extends BaseOptions {
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
    /** Fetch every page until the server reports no more results. Mutually exclusive with --count */
    all?: boolean;
    /** The number of members to return; if larger than the server page size, additional pages are fetched automatically. Defaults to the size of the first page */
    count?: number;
    /** The organization to list members for. Defaults to the user's default organization */
    org?: string;
    /** Output format. Supported values are: default and json */
    output?: string;
}

/** Options for the `pulumi org member remove` command. */
export interface PulumiOrgMemberRemoveOptions extends BaseOptions {
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
    /** The organization that owns the member */
    org?: string;
    /** Output format. Supported values are: default and json */
    output?: string;
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

/** Options for the `pulumi org usage` command. */
export interface PulumiOrgUsageOptions extends BaseOptions {
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

/** Options for the `pulumi org usage get` command. */
export interface PulumiOrgUsageGetOptions extends BaseOptions {
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
    /** Time granularity for aggregation. One of: hourly, daily, monthly */
    granularity?: string;
    /** Number of days to look back from --lookback-start (or the current time) */
    lookbackDays?: number;
    /** Unix timestamp (seconds) marking the end of the lookback window */
    lookbackStart?: number;
    /** Organization to fetch usage for (defaults to the current default org) */
    org?: string;
    /** Output format. Supported values are: default and json */
    output?: string;
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

/** Options for the `pulumi state edit` command. */
export interface PulumiStateEditOptions extends BaseOptions {
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

/** Options for the `pulumi state get` command. */
export interface PulumiStateGetOptions extends BaseOptions {
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
    /** Output format. Supported values are: default and json */
    output?: string;
    /** Show secret values in plaintext instead of [secret] */
    showSecrets?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi state move` command. */
export interface PulumiStateMoveOptions extends BaseOptions {
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
    /** The name of the stack to move resources to */
    dest?: string;
    /** Include all the parents of the moved resources as well */
    includeParents?: boolean;
    /** The name of the stack to move resources from */
    source?: string;
}

/** Options for the `pulumi state promote` command. */
export interface PulumiStatePromoteOptions extends BaseOptions {
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

/** Options for the `pulumi state protect` command. */
export interface PulumiStateProtectOptions extends BaseOptions {
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
    /** Protect all resources in the checkpoint */
    all?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi state remove` command. */
export interface PulumiStateRemoveOptions extends BaseOptions {
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
    /** Delete all resources in the stack */
    all?: boolean;
    /** Force deletion of protected resources */
    force?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Delete the URN and all its dependents */
    targetDependents?: boolean;
}

/** Options for the `pulumi state rename` command. */
export interface PulumiStateRenameOptions extends BaseOptions {
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

/** Options for the `pulumi state repair` command. */
export interface PulumiStateRepairOptions extends BaseOptions {
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

/** Options for the `pulumi state taint` command. */
export interface PulumiStateTaintOptions extends BaseOptions {
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

/** Options for the `pulumi state unprotect` command. */
export interface PulumiStateUnprotectOptions extends BaseOptions {
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
    /** Unprotect all resources in the checkpoint */
    all?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi state untaint` command. */
export interface PulumiStateUntaintOptions extends BaseOptions {
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
    /** Untaint all resources in the checkpoint */
    all?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi state upgrade` command. */
export interface PulumiStateUpgradeOptions extends BaseOptions {
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

/** Options for the `pulumi up` command. */
export interface PulumiUpOptions extends BaseOptions {
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
    /** Enable the ability to attach a debugger to the program and source based plugins being executed. Can limit debug type to 'program', 'plugins', 'plugin:<name>' or 'all'. */
    attachDebugger?: string[];
    /** The address of an existing language runtime host to connect to */
    client?: string;
    /** Config to use during the update and save to the stack config file */
    config?: string[];
    /** Use the configuration values in the specified file rather than detecting the file name */
    configFile?: string;
    /** Config keys contain a path to a property in a map or list to set */
    configPath?: boolean;
    /** Continue updating resources even if an error is encountered (can also be set with PULUMI_CONTINUE_ON_ERROR environment variable) */
    continueOnError?: boolean;
    /** [DEPRECATED] Use --neo instead. Enable Pulumi Neo's assistance for improved CLI experience and insights (can also be set with PULUMI_COPILOT environment variable) */
    copilot?: boolean;
    /** Print detailed debugging output during resource operations */
    debug?: boolean;
    /** Display operation as a rich diff showing the overall change */
    diff?: boolean;
    /** Specify a resource URN to ignore. These resources will not be updated. Multiple resources can be specified using --exclude urn1 --exclude urn2. Wildcards (*, **) are also supported */
    exclude?: string[];
    /** Allows ignoring of dependent targets discovered but not specified in --exclude list */
    excludeDependents?: boolean;
    execAgent?: string;
    execKind?: string;
    /** Return an error if any changes occur during this update. This check happens after the update is applied */
    expectNoChanges?: boolean;
    /** Ignore the protect resource option for this operation, allowing protected resources to be deleted or replaced. Use with caution: deleted resources cannot be recovered */
    ignoreProtect?: boolean;
    /** Serialize the update diffs, operations, and overall output as JSON */
    json?: boolean;
    /** Optional message to associate with the update operation */
    message?: string;
    /** Enable Pulumi Neo's assistance for improved CLI experience and insights (can also be set with PULUMI_NEO environment variable) */
    neo?: boolean;
    /** Start a Neo task to help debug errors that occur during the operation */
    neoTaskOnFailure?: boolean;
    /** Output format. Supported values are: default and json */
    output?: string;
    /** [EXPERIMENTAL] Override an imported environment for this run only, as <env>=<replacement>; repeatable */
    overrideEnv?: string[];
    /** Allow P resource operations to run in parallel at once (1 for no parallelism). */
    parallel?: number;
    /** [EXPERIMENTAL] Path to a plan file to use for the update. The update will not perform operations that exceed its plan (e.g. replacements instead of updates, or updates insteadof sames). */
    plan?: string;
    /** Run one or more policy packs as part of this update */
    policyPack?: string[];
    /** Path to JSON file containing the config for the policy pack of the corresponding "--policy-pack" flag */
    policyPackConfig?: string[];
    /** Refresh the state of the stack's resources before this update */
    refresh?: string;
    /** Specify a single resource URN to replace. Multiple resources can be specified using --replace urn1 --replace urn2. Wildcards (*, **) are also supported */
    replace?: string[];
    /** Run the program to determine up-to-date state for providers to refresh resources, this only applies if --refresh is set */
    runProgram?: boolean;
    /** The type of the provider that should be used to encrypt and decrypt secrets (possible choices: default, passphrase, awskms, azurekeyvault, gcpkms, hashivault). Only used when creating a new stack from an existing template */
    secretsProvider?: string;
    /** Show configuration keys and variables */
    showConfig?: boolean;
    /** Display full length of inputs & outputs */
    showFullOutput?: boolean;
    /** Show each exempted policy violation instead of a summary */
    showPolicyExceptions?: boolean;
    /** Show per-resource policy remediation details instead of a summary */
    showPolicyRemediations?: boolean;
    /** Show resources that are being read in, alongside those being managed directly in the stack */
    showReads?: boolean;
    /** Show detailed resource replacement creates and deletes instead of a single step */
    showReplacementSteps?: boolean;
    /** Show resources that don't need be updated because they haven't changed, alongside those that do */
    showSames?: boolean;
    /** Show secret outputs in the CLI output */
    showSecrets?: boolean;
    /** Skip validation of stack config values against the project config schema */
    skipConfigValidation?: boolean;
    /** Skip the up-front provider plugin install step; missing plugins are installed lazily by the engine. When deploying from a template, also skips installing the project's runtime dependencies. */
    skipPluginPreInstall?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** [EXPERIMENTAL] Enable strict plan behavior: generate a plan during preview and constrain the update to that plan (opt-in). Cannot be used with --skip-preview. */
    strict?: boolean;
    /** Suppress display of stack outputs (in case they contain sensitive values) */
    suppressOutputs?: boolean;
    /** Suppress display of the state permalink */
    suppressPermalink?: string;
    /** Suppress display of periodic progress dots */
    suppressProgress?: boolean;
    /** Specify a single resource URN to update. Other resources will not be updated. Multiple resources can be specified using --target urn1 --target urn2. Wildcards (*, **) are also supported */
    target?: string[];
    /** Allows updating of dependent targets discovered but not specified in --target list */
    targetDependents?: boolean;
    /** Specify a single resource URN to replace. Other resources will not be updated. Shorthand for --target urn --replace urn. */
    targetReplace?: string[];
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
