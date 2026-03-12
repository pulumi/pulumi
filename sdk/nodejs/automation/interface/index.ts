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

    aboutEnv(options: PulumiAboutEnvOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('about');
        __final.push('env');

        const __flags: string[] = [];

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    about(options: PulumiAboutOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('about');

        const __flags: string[] = [];

        if (options.json) {
            __flags.push('--json');
        }

        if (options.stack != null) {
            __flags.push('--stack', '' + options.stack);
        }

        if (options.transitive) {
            __flags.push('--transitive');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    aiWeb(options: PulumiAiWebOptions, prompt?: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('ai');
        __final.push('web');

        const __flags: string[] = [];

        if (options.language != null) {
            __flags.push('--language', '' + options.language);
        }

        if (options.noAutoSubmit) {
            __flags.push('--no-auto-submit');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (prompt != null) {
            __arguments.push('' + prompt);

        }
        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    ai(options: PulumiAiOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('ai');

        const __flags: string[] = [];

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    cancel(options: PulumiCancelOptions, stackName?: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('cancel');

        const __flags: string[] = [];

        if (options.stack != null) {
            __flags.push('--stack', '' + options.stack);
        }

        if (options.yes) {
            __flags.push('--yes');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (stackName != null) {
            __arguments.push('' + stackName);

        }
        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    configCp(options: PulumiConfigCpOptions, key?: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('config');
        __final.push('cp');

        const __flags: string[] = [];

        if (options.dest != null) {
            __flags.push('--dest', '' + options.dest);
        }

        if (options.path) {
            __flags.push('--path');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (key != null) {
            __arguments.push('' + key);

        }
        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    configEnvAdd(options: PulumiConfigEnvAddOptions, ...environmentName: string[]): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('config');
        __final.push('env');
        __final.push('add');

        const __flags: string[] = [];

        if (options.showSecrets) {
            __flags.push('--show-secrets');
        }

        if (options.yes) {
            __flags.push('--yes');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        for (const __item of environmentName ?? []) {
            __arguments.push('' + __item);

        }

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    configEnvInit(options: PulumiConfigEnvInitOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('config');
        __final.push('env');
        __final.push('init');

        const __flags: string[] = [];

        if (options.env != null) {
            __flags.push('--env', '' + options.env);
        }

        if (options.keepConfig) {
            __flags.push('--keep-config');
        }

        if (options.showSecrets) {
            __flags.push('--show-secrets');
        }

        if (options.yes) {
            __flags.push('--yes');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    configEnvLs(options: PulumiConfigEnvLsOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('config');
        __final.push('env');
        __final.push('ls');

        const __flags: string[] = [];

        if (options.json) {
            __flags.push('--json');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    configEnvRm(options: PulumiConfigEnvRmOptions, environmentName: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('config');
        __final.push('env');
        __final.push('rm');

        const __flags: string[] = [];

        if (options.showSecrets) {
            __flags.push('--show-secrets');
        }

        if (options.yes) {
            __flags.push('--yes');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        __arguments.push('' + environmentName);

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    configGet(options: PulumiConfigGetOptions, key: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('config');
        __final.push('get');

        const __flags: string[] = [];

        if (options.json) {
            __flags.push('--json');
        }

        if (options.open) {
            __flags.push('--open');
        }

        if (options.path) {
            __flags.push('--path');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        __arguments.push('' + key);

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    configRefresh(options: PulumiConfigRefreshOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('config');
        __final.push('refresh');

        const __flags: string[] = [];

        if (options.force) {
            __flags.push('--force');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    configRm(options: PulumiConfigRmOptions, key: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('config');
        __final.push('rm');

        const __flags: string[] = [];

        if (options.path) {
            __flags.push('--path');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        __arguments.push('' + key);

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    configRmAll(options: PulumiConfigRmAllOptions, ...key: string[]): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('config');
        __final.push('rm-all');

        const __flags: string[] = [];

        if (options.path) {
            __flags.push('--path');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        for (const __item of key ?? []) {
            __arguments.push('' + __item);

        }

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    configSet(options: PulumiConfigSetOptions, key: string, value?: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('config');
        __final.push('set');

        const __flags: string[] = [];

        if (options.path) {
            __flags.push('--path');
        }

        if (options.plaintext) {
            __flags.push('--plaintext');
        }

        if (options.secret) {
            __flags.push('--secret');
        }

        if (options.type != null) {
            __flags.push('--type', '' + options.type);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        __arguments.push('' + key);

        if (value != null) {
            __arguments.push('' + value);

        }
        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    configSetAll(options: PulumiConfigSetAllOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('config');
        __final.push('set-all');

        const __flags: string[] = [];

        if (options.json != null) {
            __flags.push('--json', '' + options.json);
        }

        if (options.path) {
            __flags.push('--path');
        }

        for (const __item of options.plaintext ?? []) {
            if (__item != null) {
                __flags.push('--plaintext', '' + __item);
            }

        }

        for (const __item of options.secret ?? []) {
            if (__item != null) {
                __flags.push('--secret', '' + __item);
            }

        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    config(options: PulumiConfigOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('config');

        const __flags: string[] = [];

        if (options.configFile != null) {
            __flags.push('--config-file', '' + options.configFile);
        }

        if (options.json) {
            __flags.push('--json');
        }

        if (options.open) {
            __flags.push('--open');
        }

        if (options.showSecrets) {
            __flags.push('--show-secrets');
        }

        if (options.stack != null) {
            __flags.push('--stack', '' + options.stack);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    console(options: PulumiConsoleOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('console');

        const __flags: string[] = [];

        if (options.stack != null) {
            __flags.push('--stack', '' + options.stack);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    convert(options: PulumiConvertOptions, ...arg: string[]): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('convert');

        const __flags: string[] = [];

        if (options.from != null) {
            __flags.push('--from', '' + options.from);
        }

        if (options.generateOnly) {
            __flags.push('--generate-only');
        }

        __flags.push('--language', '' + options.language);

        for (const __item of options.mappings ?? []) {
            if (__item != null) {
                __flags.push('--mappings', '' + __item);
            }

        }

        if (options.name != null) {
            __flags.push('--name', '' + options.name);
        }

        if (options.out != null) {
            __flags.push('--out', '' + options.out);
        }

        if (options.strict) {
            __flags.push('--strict');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (arg != null) {
            for (const __item of arg ?? []) {
                __arguments.push('' + __item);

            }

        }
        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    convertTrace(options: PulumiConvertTraceOptions, traceFile: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('convert-trace');

        const __flags: string[] = [];

        if (options.granularity != null) {
            __flags.push('--granularity', '' + options.granularity);
        }

        if (options.ignoreLogSpans) {
            __flags.push('--ignore-log-spans');
        }

        if (options.otel) {
            __flags.push('--otel');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        __arguments.push('' + traceFile);

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    deploymentRun(options: PulumiDeploymentRunOptions, operation: string, url?: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('deployment');
        __final.push('run');

        const __flags: string[] = [];

        if (options.agentPoolId != null) {
            __flags.push('--agent-pool-id', '' + options.agentPoolId);
        }

        for (const __item of options.env ?? []) {
            if (__item != null) {
                __flags.push('--env', '' + __item);
            }

        }

        for (const __item of options.envSecret ?? []) {
            if (__item != null) {
                __flags.push('--env-secret', '' + __item);
            }

        }

        if (options.executorImage != null) {
            __flags.push('--executor-image', '' + options.executorImage);
        }

        if (options.executorImagePassword != null) {
            __flags.push('--executor-image-password', '' + options.executorImagePassword);
        }

        if (options.executorImageUsername != null) {
            __flags.push('--executor-image-username', '' + options.executorImageUsername);
        }

        if (options.gitAuthAccessToken != null) {
            __flags.push('--git-auth-access-token', '' + options.gitAuthAccessToken);
        }

        if (options.gitAuthPassword != null) {
            __flags.push('--git-auth-password', '' + options.gitAuthPassword);
        }

        if (options.gitAuthSshPrivateKey != null) {
            __flags.push('--git-auth-ssh-private-key', '' + options.gitAuthSshPrivateKey);
        }

        if (options.gitAuthSshPrivateKeyPath != null) {
            __flags.push('--git-auth-ssh-private-key-path', '' + options.gitAuthSshPrivateKeyPath);
        }

        if (options.gitAuthUsername != null) {
            __flags.push('--git-auth-username', '' + options.gitAuthUsername);
        }

        if (options.gitBranch != null) {
            __flags.push('--git-branch', '' + options.gitBranch);
        }

        if (options.gitCommit != null) {
            __flags.push('--git-commit', '' + options.gitCommit);
        }

        if (options.gitRepoDir != null) {
            __flags.push('--git-repo-dir', '' + options.gitRepoDir);
        }

        if (options.inheritSettings) {
            __flags.push('--inherit-settings');
        }

        for (const __item of options.preRunCommand ?? []) {
            if (__item != null) {
                __flags.push('--pre-run-command', '' + __item);
            }

        }

        if (options.skipInstallDependencies) {
            __flags.push('--skip-install-dependencies');
        }

        if (options.stack != null) {
            __flags.push('--stack', '' + options.stack);
        }

        if (options.suppressPermalink) {
            __flags.push('--suppress-permalink');
        }

        if (options.suppressStreamLogs) {
            __flags.push('--suppress-stream-logs');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        __arguments.push('' + operation);

        if (url != null) {
            __arguments.push('' + url);

        }
        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    deploymentSettingsConfigure(options: PulumiDeploymentSettingsConfigureOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('deployment');
        __final.push('settings');
        __final.push('configure');

        const __flags: string[] = [];

        if (options.gitAuthSshPrivateKey != null) {
            __flags.push('--git-auth-ssh-private-key', '' + options.gitAuthSshPrivateKey);
        }

        if (options.gitAuthSshPrivateKeyPath != null) {
            __flags.push('--git-auth-ssh-private-key-path', '' + options.gitAuthSshPrivateKeyPath);
        }

        if (options.stack != null) {
            __flags.push('--stack', '' + options.stack);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    deploymentSettingsDestroy(options: PulumiDeploymentSettingsDestroyOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('deployment');
        __final.push('settings');
        __final.push('destroy');

        const __flags: string[] = [];

        if (options.stack != null) {
            __flags.push('--stack', '' + options.stack);
        }

        if (options.yes) {
            __flags.push('--yes');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    deploymentSettingsEnv(options: PulumiDeploymentSettingsEnvOptions, key: string, value?: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('deployment');
        __final.push('settings');
        __final.push('env');

        const __flags: string[] = [];

        if (options.remove) {
            __flags.push('--remove');
        }

        if (options.secret) {
            __flags.push('--secret');
        }

        if (options.stack != null) {
            __flags.push('--stack', '' + options.stack);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        __arguments.push('' + key);

        if (value != null) {
            __arguments.push('' + value);

        }
        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    deploymentSettingsInit(options: PulumiDeploymentSettingsInitOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('deployment');
        __final.push('settings');
        __final.push('init');

        const __flags: string[] = [];

        if (options.force) {
            __flags.push('--force');
        }

        if (options.gitAuthSshPrivateKey != null) {
            __flags.push('--git-auth-ssh-private-key', '' + options.gitAuthSshPrivateKey);
        }

        if (options.gitAuthSshPrivateKeyPath != null) {
            __flags.push('--git-auth-ssh-private-key-path', '' + options.gitAuthSshPrivateKeyPath);
        }

        if (options.stack != null) {
            __flags.push('--stack', '' + options.stack);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    deploymentSettingsPull(options: PulumiDeploymentSettingsPullOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('deployment');
        __final.push('settings');
        __final.push('pull');

        const __flags: string[] = [];

        if (options.stack != null) {
            __flags.push('--stack', '' + options.stack);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    deploymentSettingsPush(options: PulumiDeploymentSettingsPushOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('deployment');
        __final.push('settings');
        __final.push('push');

        const __flags: string[] = [];

        if (options.stack != null) {
            __flags.push('--stack', '' + options.stack);
        }

        if (options.yes) {
            __flags.push('--yes');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    deploymentSettings(options: PulumiDeploymentSettingsOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('deployment');
        __final.push('settings');

        const __flags: string[] = [];

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    deployment(options: PulumiDeploymentOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('deployment');

        const __flags: string[] = [];

        if (options.configFile != null) {
            __flags.push('--config-file', '' + options.configFile);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    destroy(options: PulumiDestroyOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('destroy');

        const __flags: string[] = [];

        if (options.client != null) {
            __flags.push('--client', '' + options.client);
        }

        for (const __item of options.config ?? []) {
            if (__item != null) {
                __flags.push('--config', '' + __item);
            }

        }

        if (options.configFile != null) {
            __flags.push('--config-file', '' + options.configFile);
        }

        if (options.configPath) {
            __flags.push('--config-path');
        }

        if (options.continueOnError) {
            __flags.push('--continue-on-error');
        }

        if (options.copilot) {
            __flags.push('--copilot');
        }

        if (options.debug) {
            __flags.push('--debug');
        }

        if (options.diff) {
            __flags.push('--diff');
        }

        for (const __item of options.exclude ?? []) {
            if (__item != null) {
                __flags.push('--exclude', '' + __item);
            }

        }

        if (options.excludeProtected) {
            __flags.push('--exclude-protected');
        }

        if (options.execAgent != null) {
            __flags.push('--exec-agent', '' + options.execAgent);
        }

        if (options.execKind != null) {
            __flags.push('--exec-kind', '' + options.execKind);
        }

        if (options.json) {
            __flags.push('--json');
        }

        if (options.message != null) {
            __flags.push('--message', '' + options.message);
        }

        if (options.neo) {
            __flags.push('--neo');
        }

        if (options.parallel != null) {
            __flags.push('--parallel', '' + options.parallel);
        }

        if (options.previewOnly) {
            __flags.push('--preview-only');
        }

        if (options.refresh != null) {
            __flags.push('--refresh', '' + options.refresh);
        }

        if (options.remove) {
            __flags.push('--remove');
        }

        if (options.runProgram) {
            __flags.push('--run-program');
        }

        if (options.showConfig) {
            __flags.push('--show-config');
        }

        if (options.showFullOutput) {
            __flags.push('--show-full-output');
        }

        if (options.showReplacementSteps) {
            __flags.push('--show-replacement-steps');
        }

        if (options.showSames) {
            __flags.push('--show-sames');
        }

        if (options.skipPreview) {
            __flags.push('--skip-preview');
        }

        if (options.stack != null) {
            __flags.push('--stack', '' + options.stack);
        }

        if (options.suppressOutputs) {
            __flags.push('--suppress-outputs');
        }

        if (options.suppressPermalink != null) {
            __flags.push('--suppress-permalink', '' + options.suppressPermalink);
        }

        if (options.suppressProgress) {
            __flags.push('--suppress-progress');
        }

        for (const __item of options.target ?? []) {
            if (__item != null) {
                __flags.push('--target', '' + __item);
            }

        }

        if (options.targetDependents) {
            __flags.push('--target-dependents');
        }

        if (options.yes) {
            __flags.push('--yes');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    envClone(options: PulumiEnvCloneOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('env');
        __final.push('clone');

        const __flags: string[] = [];

        if (options.preserveAccess) {
            __flags.push('--preserve-access');
        }

        if (options.preserveEnvTags) {
            __flags.push('--preserve-env-tags');
        }

        if (options.preserveHistory) {
            __flags.push('--preserve-history');
        }

        if (options.preserveRevTags) {
            __flags.push('--preserve-rev-tags');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    envDiff(options: PulumiEnvDiffOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('env');
        __final.push('diff');

        const __flags: string[] = [];

        if (options.format != null) {
            __flags.push('--format', '' + options.format);
        }

        if (options.path != null) {
            __flags.push('--path', '' + options.path);
        }

        if (options.showSecrets) {
            __flags.push('--show-secrets');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    envEdit(options: PulumiEnvEditOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('env');
        __final.push('edit');

        const __flags: string[] = [];

        if (options.draft != null) {
            __flags.push('--draft', '' + options.draft);
        }

        if (options.editor != null) {
            __flags.push('--editor', '' + options.editor);
        }

        if (options.file != null) {
            __flags.push('--file', '' + options.file);
        }

        if (options.showSecrets) {
            __flags.push('--show-secrets');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    envGet(options: PulumiEnvGetOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('env');
        __final.push('get');

        const __flags: string[] = [];

        if (options.definition) {
            __flags.push('--definition');
        }

        if (options.showSecrets) {
            __flags.push('--show-secrets');
        }

        if (options.value != null) {
            __flags.push('--value', '' + options.value);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    envInit(options: PulumiEnvInitOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('env');
        __final.push('init');

        const __flags: string[] = [];

        if (options.file != null) {
            __flags.push('--file', '' + options.file);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    envLs(options: PulumiEnvLsOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('env');
        __final.push('ls');

        const __flags: string[] = [];

        if (options.organization != null) {
            __flags.push('--organization', '' + options.organization);
        }

        if (options.project != null) {
            __flags.push('--project', '' + options.project);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    envOpen(options: PulumiEnvOpenOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('env');
        __final.push('open');

        const __flags: string[] = [];

        if (options.draft != null) {
            __flags.push('--draft', '' + options.draft);
        }

        if (options.format != null) {
            __flags.push('--format', '' + options.format);
        }

        if (options.lifetime != null) {
            __flags.push('--lifetime', '' + options.lifetime);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    envRm(options: PulumiEnvRmOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('env');
        __final.push('rm');

        const __flags: string[] = [];

        if (options.yes) {
            __flags.push('--yes');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    envRotate(options: PulumiEnvRotateOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('env');
        __final.push('rotate');

        const __flags: string[] = [];

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    envRun(options: PulumiEnvRunOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('env');
        __final.push('run');

        const __flags: string[] = [];

        if (options.draft != null) {
            __flags.push('--draft', '' + options.draft);
        }

        if (options.interactive) {
            __flags.push('--interactive');
        }

        if (options.lifetime != null) {
            __flags.push('--lifetime', '' + options.lifetime);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    envSet(options: PulumiEnvSetOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('env');
        __final.push('set');

        const __flags: string[] = [];

        if (options.draft != null) {
            __flags.push('--draft', '' + options.draft);
        }

        if (options.file != null) {
            __flags.push('--file', '' + options.file);
        }

        if (options.plaintext) {
            __flags.push('--plaintext');
        }

        if (options.secret) {
            __flags.push('--secret');
        }

        if (options.string) {
            __flags.push('--string');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    envTagGet(options: PulumiEnvTagGetOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('env');
        __final.push('tag');
        __final.push('get');

        const __flags: string[] = [];

        if (options.utc) {
            __flags.push('--utc');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    envTagLs(options: PulumiEnvTagLsOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('env');
        __final.push('tag');
        __final.push('ls');

        const __flags: string[] = [];

        if (options.pager != null) {
            __flags.push('--pager', '' + options.pager);
        }

        if (options.utc) {
            __flags.push('--utc');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    envTagMv(options: PulumiEnvTagMvOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('env');
        __final.push('tag');
        __final.push('mv');

        const __flags: string[] = [];

        if (options.utc) {
            __flags.push('--utc');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    envTagRm(options: PulumiEnvTagRmOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('env');
        __final.push('tag');
        __final.push('rm');

        const __flags: string[] = [];

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    envTag(options: PulumiEnvTagOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('env');
        __final.push('tag');

        const __flags: string[] = [];

        if (options.utc) {
            __flags.push('--utc');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    envVersionHistory(options: PulumiEnvVersionHistoryOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('env');
        __final.push('version');
        __final.push('history');

        const __flags: string[] = [];

        if (options.pager != null) {
            __flags.push('--pager', '' + options.pager);
        }

        if (options.utc) {
            __flags.push('--utc');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    envVersionRetract(options: PulumiEnvVersionRetractOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('env');
        __final.push('version');
        __final.push('retract');

        const __flags: string[] = [];

        if (options.reason != null) {
            __flags.push('--reason', '' + options.reason);
        }

        if (options.replaceWith != null) {
            __flags.push('--replace-with', '' + options.replaceWith);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    envVersionRollback(options: PulumiEnvVersionRollbackOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('env');
        __final.push('version');
        __final.push('rollback');

        const __flags: string[] = [];

        if (options.draft != null) {
            __flags.push('--draft', '' + options.draft);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    envVersionTagLs(options: PulumiEnvVersionTagLsOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('env');
        __final.push('version');
        __final.push('tag');
        __final.push('ls');

        const __flags: string[] = [];

        if (options.pager != null) {
            __flags.push('--pager', '' + options.pager);
        }

        if (options.utc) {
            __flags.push('--utc');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    envVersionTagRm(options: PulumiEnvVersionTagRmOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('env');
        __final.push('version');
        __final.push('tag');
        __final.push('rm');

        const __flags: string[] = [];

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    envVersionTag(options: PulumiEnvVersionTagOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('env');
        __final.push('version');
        __final.push('tag');

        const __flags: string[] = [];

        if (options.utc) {
            __flags.push('--utc');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    envVersion(options: PulumiEnvVersionOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('env');
        __final.push('version');

        const __flags: string[] = [];

        if (options.utc) {
            __flags.push('--utc');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    genCompletion(options: PulumiGenCompletionOptions, shell: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('gen-completion');

        const __flags: string[] = [];

        __final.push(... __flags);

        const __arguments: string[] = [];

        __arguments.push('' + shell);

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    genMarkdown(options: PulumiGenMarkdownOptions, dir: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('gen-markdown');

        const __flags: string[] = [];

        __final.push(... __flags);

        const __arguments: string[] = [];

        __arguments.push('' + dir);

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    generateCliSpec(options: PulumiGenerateCliSpecOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('generate-cli-spec');

        const __flags: string[] = [];

        if (options.help) {
            __flags.push('--help');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    help(options: PulumiHelpOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('help');

        const __flags: string[] = [];

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    import(options: PulumiImportOptions, ...arg: string[]): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('import');

        const __flags: string[] = [];

        if (options.configFile != null) {
            __flags.push('--config-file', '' + options.configFile);
        }

        if (options.debug) {
            __flags.push('--debug');
        }

        if (options.diff) {
            __flags.push('--diff');
        }

        if (options.execAgent != null) {
            __flags.push('--exec-agent', '' + options.execAgent);
        }

        if (options.execKind != null) {
            __flags.push('--exec-kind', '' + options.execKind);
        }

        if (options.file != null) {
            __flags.push('--file', '' + options.file);
        }

        if (options.from != null) {
            __flags.push('--from', '' + options.from);
        }

        if (options.generateCode) {
            __flags.push('--generate-code');
        }

        if (options.generateResources != null) {
            __flags.push('--generate-resources', '' + options.generateResources);
        }

        if (options.json) {
            __flags.push('--json');
        }

        if (options.message != null) {
            __flags.push('--message', '' + options.message);
        }

        if (options.out != null) {
            __flags.push('--out', '' + options.out);
        }

        if (options.parallel != null) {
            __flags.push('--parallel', '' + options.parallel);
        }

        if (options.parent != null) {
            __flags.push('--parent', '' + options.parent);
        }

        if (options.previewOnly) {
            __flags.push('--preview-only');
        }

        for (const __item of options.properties ?? []) {
            if (__item != null) {
                __flags.push('--properties', '' + __item);
            }

        }

        if (options.protect) {
            __flags.push('--protect');
        }

        if (options.provider != null) {
            __flags.push('--provider', '' + options.provider);
        }

        if (options.skipPreview) {
            __flags.push('--skip-preview');
        }

        if (options.stack != null) {
            __flags.push('--stack', '' + options.stack);
        }

        if (options.suppressOutputs) {
            __flags.push('--suppress-outputs');
        }

        if (options.suppressPermalink != null) {
            __flags.push('--suppress-permalink', '' + options.suppressPermalink);
        }

        if (options.suppressProgress) {
            __flags.push('--suppress-progress');
        }

        if (options.yes) {
            __flags.push('--yes');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (arg != null) {
            for (const __item of arg ?? []) {
                __arguments.push('' + __item);

            }

        }
        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    install(options: PulumiInstallOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('install');

        const __flags: string[] = [];

        if (options.noDependencies) {
            __flags.push('--no-dependencies');
        }

        if (options.noPlugins) {
            __flags.push('--no-plugins');
        }

        if (options.parallel != null) {
            __flags.push('--parallel', '' + options.parallel);
        }

        if (options.reinstall) {
            __flags.push('--reinstall');
        }

        if (options.useLanguageVersionTools) {
            __flags.push('--use-language-version-tools');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    login(options: PulumiLoginOptions, url?: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('login');

        const __flags: string[] = [];

        if (options.cloudUrl != null) {
            __flags.push('--cloud-url', '' + options.cloudUrl);
        }

        if (options.defaultOrg != null) {
            __flags.push('--default-org', '' + options.defaultOrg);
        }

        if (options.insecure) {
            __flags.push('--insecure');
        }

        if (options.interactive) {
            __flags.push('--interactive');
        }

        if (options.local) {
            __flags.push('--local');
        }

        if (options.oidcExpiration != null) {
            __flags.push('--oidc-expiration', '' + options.oidcExpiration);
        }

        if (options.oidcOrg != null) {
            __flags.push('--oidc-org', '' + options.oidcOrg);
        }

        if (options.oidcTeam != null) {
            __flags.push('--oidc-team', '' + options.oidcTeam);
        }

        if (options.oidcToken != null) {
            __flags.push('--oidc-token', '' + options.oidcToken);
        }

        if (options.oidcUser != null) {
            __flags.push('--oidc-user', '' + options.oidcUser);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (url != null) {
            __arguments.push('' + url);

        }
        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    logout(options: PulumiLogoutOptions, url?: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('logout');

        const __flags: string[] = [];

        if (options.all) {
            __flags.push('--all');
        }

        if (options.cloudUrl != null) {
            __flags.push('--cloud-url', '' + options.cloudUrl);
        }

        if (options.local) {
            __flags.push('--local');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (url != null) {
            __arguments.push('' + url);

        }
        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    logs(options: PulumiLogsOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('logs');

        const __flags: string[] = [];

        if (options.configFile != null) {
            __flags.push('--config-file', '' + options.configFile);
        }

        if (options.follow) {
            __flags.push('--follow');
        }

        if (options.json) {
            __flags.push('--json');
        }

        if (options.resource != null) {
            __flags.push('--resource', '' + options.resource);
        }

        if (options.since != null) {
            __flags.push('--since', '' + options.since);
        }

        if (options.stack != null) {
            __flags.push('--stack', '' + options.stack);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    new(options: PulumiNewOptions, templateOrUrl?: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('new');

        const __flags: string[] = [];

        if (options.ai != null) {
            __flags.push('--ai', '' + options.ai);
        }

        for (const __item of options.config ?? []) {
            if (__item != null) {
                __flags.push('--config', '' + __item);
            }

        }

        if (options.configPath) {
            __flags.push('--config-path');
        }

        if (options.description != null) {
            __flags.push('--description', '' + options.description);
        }

        if (options.dir != null) {
            __flags.push('--dir', '' + options.dir);
        }

        if (options.force) {
            __flags.push('--force');
        }

        if (options.generateOnly) {
            __flags.push('--generate-only');
        }

        if (options.language != null) {
            __flags.push('--language', '' + options.language);
        }

        if (options.listTemplates) {
            __flags.push('--list-templates');
        }

        if (options.name != null) {
            __flags.push('--name', '' + options.name);
        }

        if (options.offline) {
            __flags.push('--offline');
        }

        if (options.remoteStackConfig) {
            __flags.push('--remote-stack-config');
        }

        for (const __item of options.runtimeOptions ?? []) {
            if (__item != null) {
                __flags.push('--runtime-options', '' + __item);
            }

        }

        if (options.secretsProvider != null) {
            __flags.push('--secrets-provider', '' + options.secretsProvider);
        }

        if (options.stack != null) {
            __flags.push('--stack', '' + options.stack);
        }

        if (options.templateMode) {
            __flags.push('--template-mode');
        }

        if (options.yes) {
            __flags.push('--yes');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (templateOrUrl != null) {
            __arguments.push('' + templateOrUrl);

        }
        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    orgGetDefault(options: PulumiOrgGetDefaultOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('org');
        __final.push('get-default');

        const __flags: string[] = [];

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    orgSearchAi(options: PulumiOrgSearchAiOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('org');
        __final.push('search');
        __final.push('ai');

        const __flags: string[] = [];

        if (options.delimiter != null) {
            __flags.push('--delimiter', '' + options.delimiter);
        }

        if (options.org != null) {
            __flags.push('--org', '' + options.org);
        }

        if (options.output != null) {
            __flags.push('--output', '' + options.output);
        }

        if (options.query != null) {
            __flags.push('--query', '' + options.query);
        }

        if (options.web) {
            __flags.push('--web');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    orgSearch(options: PulumiOrgSearchOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('org');
        __final.push('search');

        const __flags: string[] = [];

        if (options.delimiter != null) {
            __flags.push('--delimiter', '' + options.delimiter);
        }

        if (options.org != null) {
            __flags.push('--org', '' + options.org);
        }

        if (options.output != null) {
            __flags.push('--output', '' + options.output);
        }

        for (const __item of options.query ?? []) {
            if (__item != null) {
                __flags.push('--query', '' + __item);
            }

        }

        if (options.web) {
            __flags.push('--web');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    orgSetDefault(options: PulumiOrgSetDefaultOptions, name: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('org');
        __final.push('set-default');

        const __flags: string[] = [];

        __final.push(... __flags);

        const __arguments: string[] = [];

        __arguments.push('' + name);

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    org(options: PulumiOrgOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('org');

        const __flags: string[] = [];

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    packageAdd(options: PulumiPackageAddOptions, provider: string, ...providerParameter: string[]): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('package');
        __final.push('add');

        const __flags: string[] = [];

        __final.push(... __flags);

        const __arguments: string[] = [];

        for (const __item of provider ?? []) {
            __arguments.push('' + __item);

        }

        if (providerParameter != null) {
            for (const __item of providerParameter ?? []) {
                __arguments.push('' + __item);

            }

        }
        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    packageDelete(options: PulumiPackageDeleteOptions, package_: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('package');
        __final.push('delete');

        const __flags: string[] = [];

        if (options.yes) {
            __flags.push('--yes');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        __arguments.push('' + package_);

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    packageGenSdk(options: PulumiPackageGenSdkOptions, schemaSource: string, ...providerParameter: string[]): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('package');
        __final.push('gen-sdk');

        const __flags: string[] = [];

        if (options.language != null) {
            __flags.push('--language', '' + options.language);
        }

        if (options.local) {
            __flags.push('--local');
        }

        if (options.out != null) {
            __flags.push('--out', '' + options.out);
        }

        if (options.overlays != null) {
            __flags.push('--overlays', '' + options.overlays);
        }

        if (options.version != null) {
            __flags.push('--version', '' + options.version);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        for (const __item of schemaSource ?? []) {
            __arguments.push('' + __item);

        }

        if (providerParameter != null) {
            for (const __item of providerParameter ?? []) {
                __arguments.push('' + __item);

            }

        }
        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    packageGetMapping(options: PulumiPackageGetMappingOptions, key: string, schemaSource: string, providerKey?: string, ...providerParameter: string[]): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('package');
        __final.push('get-mapping');

        const __flags: string[] = [];

        if (options.out != null) {
            __flags.push('--out', '' + options.out);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        for (const __item of key ?? []) {
            __arguments.push('' + __item);

        }

        for (const __item of schemaSource ?? []) {
            __arguments.push('' + __item);

        }

        if (providerKey != null) {
            for (const __item of providerKey ?? []) {
                __arguments.push('' + __item);

            }

        }
        if (providerParameter != null) {
            for (const __item of providerParameter ?? []) {
                __arguments.push('' + __item);

            }

        }
        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    packageGetSchema(options: PulumiPackageGetSchemaOptions, schemaSource: string, ...providerParameter: string[]): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('package');
        __final.push('get-schema');

        const __flags: string[] = [];

        __final.push(... __flags);

        const __arguments: string[] = [];

        for (const __item of schemaSource ?? []) {
            __arguments.push('' + __item);

        }

        if (providerParameter != null) {
            for (const __item of providerParameter ?? []) {
                __arguments.push('' + __item);

            }

        }
        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    packageInfo(options: PulumiPackageInfoOptions, provider: string, ...providerParameter: string[]): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('package');
        __final.push('info');

        const __flags: string[] = [];

        if (options.function != null) {
            __flags.push('--function', '' + options.function);
        }

        if (options.module != null) {
            __flags.push('--module', '' + options.module);
        }

        if (options.resource != null) {
            __flags.push('--resource', '' + options.resource);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        for (const __item of provider ?? []) {
            __arguments.push('' + __item);

        }

        if (providerParameter != null) {
            for (const __item of providerParameter ?? []) {
                __arguments.push('' + __item);

            }

        }
        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    packagePackSdk(options: PulumiPackagePackSdkOptions, language: string, path: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('package');
        __final.push('pack-sdk');

        const __flags: string[] = [];

        __final.push(... __flags);

        const __arguments: string[] = [];

        __arguments.push('' + language);

        __arguments.push('' + path);

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    packagePublish(options: PulumiPackagePublishOptions, provider: string, ...providerParameter: string[]): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('package');
        __final.push('publish');

        const __flags: string[] = [];

        if (options.installationConfiguration != null) {
            __flags.push('--installation-configuration', '' + options.installationConfiguration);
        }

        if (options.publisher != null) {
            __flags.push('--publisher', '' + options.publisher);
        }

        if (options.readme != null) {
            __flags.push('--readme', '' + options.readme);
        }

        if (options.source != null) {
            __flags.push('--source', '' + options.source);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        for (const __item of provider ?? []) {
            __arguments.push('' + __item);

        }

        if (providerParameter != null) {
            for (const __item of providerParameter ?? []) {
                __arguments.push('' + __item);

            }

        }
        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    packagePublishSdk(options: PulumiPackagePublishSdkOptions, language?: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('package');
        __final.push('publish-sdk');

        const __flags: string[] = [];

        if (options.path != null) {
            __flags.push('--path', '' + options.path);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (language != null) {
            __arguments.push('' + language);

        }
        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    pluginInstall(options: PulumiPluginInstallOptions, kind?: string, name?: string, version?: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('plugin');
        __final.push('install');

        const __flags: string[] = [];

        if (options.checksum != null) {
            __flags.push('--checksum', '' + options.checksum);
        }

        if (options.exact) {
            __flags.push('--exact');
        }

        if (options.file != null) {
            __flags.push('--file', '' + options.file);
        }

        if (options.reinstall) {
            __flags.push('--reinstall');
        }

        if (options.server != null) {
            __flags.push('--server', '' + options.server);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (kind != null) {
            __arguments.push('' + kind);

        }
        if (name != null) {
            __arguments.push('' + name);

        }
        if (version != null) {
            __arguments.push('' + version);

        }
        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    pluginLs(options: PulumiPluginLsOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('plugin');
        __final.push('ls');

        const __flags: string[] = [];

        if (options.json) {
            __flags.push('--json');
        }

        if (options.project) {
            __flags.push('--project');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    pluginRm(options: PulumiPluginRmOptions, kind?: string, name?: string, version?: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('plugin');
        __final.push('rm');

        const __flags: string[] = [];

        if (options.all) {
            __flags.push('--all');
        }

        if (options.yes) {
            __flags.push('--yes');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (kind != null) {
            __arguments.push('' + kind);

        }
        if (name != null) {
            __arguments.push('' + name);

        }
        if (version != null) {
            __arguments.push('' + version);

        }
        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    pluginRun(options: PulumiPluginRunOptions, nameOrPath: string, ...args: string[]): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('plugin');
        __final.push('run');

        const __flags: string[] = [];

        if (options.kind != null) {
            __flags.push('--kind', '' + options.kind);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        for (const __item of nameOrPath ?? []) {
            __arguments.push('' + __item);

        }

        if (args != null) {
            for (const __item of args ?? []) {
                __arguments.push('' + __item);

            }

        }
        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    policyDisable(options: PulumiPolicyDisableOptions, policyPack: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('policy');
        __final.push('disable');

        const __flags: string[] = [];

        if (options.policyGroup != null) {
            __flags.push('--policy-group', '' + options.policyGroup);
        }

        if (options.version != null) {
            __flags.push('--version', '' + options.version);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        __arguments.push('' + policyPack);

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    policyEnable(options: PulumiPolicyEnableOptions, policyPack: string, version: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('policy');
        __final.push('enable');

        const __flags: string[] = [];

        if (options.config != null) {
            __flags.push('--config', '' + options.config);
        }

        if (options.policyGroup != null) {
            __flags.push('--policy-group', '' + options.policyGroup);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        __arguments.push('' + policyPack);

        __arguments.push('' + version);

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    policyGroupLs(options: PulumiPolicyGroupLsOptions, orgName?: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('policy');
        __final.push('group');
        __final.push('ls');

        const __flags: string[] = [];

        if (options.json) {
            __flags.push('--json');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (orgName != null) {
            __arguments.push('' + orgName);

        }
        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    policyInstall(options: PulumiPolicyInstallOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('policy');
        __final.push('install');

        const __flags: string[] = [];

        if (options.stack != null) {
            __flags.push('--stack', '' + options.stack);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    policyLs(options: PulumiPolicyLsOptions, orgName?: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('policy');
        __final.push('ls');

        const __flags: string[] = [];

        if (options.json) {
            __flags.push('--json');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (orgName != null) {
            __arguments.push('' + orgName);

        }
        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    policyNew(options: PulumiPolicyNewOptions, template?: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('policy');
        __final.push('new');

        const __flags: string[] = [];

        if (options.dir != null) {
            __flags.push('--dir', '' + options.dir);
        }

        if (options.force) {
            __flags.push('--force');
        }

        if (options.generateOnly) {
            __flags.push('--generate-only');
        }

        if (options.offline) {
            __flags.push('--offline');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (template != null) {
            __arguments.push('' + template);

        }
        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    policyPublish(options: PulumiPolicyPublishOptions, orgName?: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('policy');
        __final.push('publish');

        const __flags: string[] = [];

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (orgName != null) {
            __arguments.push('' + orgName);

        }
        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    policyRm(options: PulumiPolicyRmOptions, policyPack: string, version: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('policy');
        __final.push('rm');

        const __flags: string[] = [];

        if (options.yes) {
            __flags.push('--yes');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        __arguments.push('' + policyPack);

        __arguments.push('' + version);

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    policyValidateConfig(options: PulumiPolicyValidateConfigOptions, policyPack: string, version: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('policy');
        __final.push('validate-config');

        const __flags: string[] = [];

        __flags.push('--config', '' + options.config);

        __final.push(... __flags);

        const __arguments: string[] = [];

        __arguments.push('' + policyPack);

        __arguments.push('' + version);

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    preview(options: PulumiPreviewOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('preview');

        const __flags: string[] = [];

        for (const __item of options.attachDebugger ?? []) {
            if (__item != null) {
                __flags.push('--attach-debugger', '' + __item);
            }

        }

        if (options.client != null) {
            __flags.push('--client', '' + options.client);
        }

        for (const __item of options.config ?? []) {
            if (__item != null) {
                __flags.push('--config', '' + __item);
            }

        }

        if (options.configFile != null) {
            __flags.push('--config-file', '' + options.configFile);
        }

        if (options.configPath) {
            __flags.push('--config-path');
        }

        if (options.copilot) {
            __flags.push('--copilot');
        }

        if (options.debug) {
            __flags.push('--debug');
        }

        if (options.diff) {
            __flags.push('--diff');
        }

        for (const __item of options.exclude ?? []) {
            if (__item != null) {
                __flags.push('--exclude', '' + __item);
            }

        }

        if (options.excludeDependents) {
            __flags.push('--exclude-dependents');
        }

        if (options.execAgent != null) {
            __flags.push('--exec-agent', '' + options.execAgent);
        }

        if (options.execKind != null) {
            __flags.push('--exec-kind', '' + options.execKind);
        }

        if (options.expectNoChanges) {
            __flags.push('--expect-no-changes');
        }

        if (options.importFile != null) {
            __flags.push('--import-file', '' + options.importFile);
        }

        if (options.json) {
            __flags.push('--json');
        }

        if (options.message != null) {
            __flags.push('--message', '' + options.message);
        }

        if (options.neo) {
            __flags.push('--neo');
        }

        if (options.neoTaskOnFailure) {
            __flags.push('--neo-task-on-failure');
        }

        if (options.parallel != null) {
            __flags.push('--parallel', '' + options.parallel);
        }

        for (const __item of options.policyPack ?? []) {
            if (__item != null) {
                __flags.push('--policy-pack', '' + __item);
            }

        }

        for (const __item of options.policyPackConfig ?? []) {
            if (__item != null) {
                __flags.push('--policy-pack-config', '' + __item);
            }

        }

        if (options.refresh != null) {
            __flags.push('--refresh', '' + options.refresh);
        }

        for (const __item of options.replace ?? []) {
            if (__item != null) {
                __flags.push('--replace', '' + __item);
            }

        }

        if (options.runProgram) {
            __flags.push('--run-program');
        }

        if (options.savePlan != null) {
            __flags.push('--save-plan', '' + options.savePlan);
        }

        if (options.showConfig) {
            __flags.push('--show-config');
        }

        if (options.showFullOutput) {
            __flags.push('--show-full-output');
        }

        if (options.showPolicyRemediations) {
            __flags.push('--show-policy-remediations');
        }

        if (options.showReads) {
            __flags.push('--show-reads');
        }

        if (options.showReplacementSteps) {
            __flags.push('--show-replacement-steps');
        }

        if (options.showSames) {
            __flags.push('--show-sames');
        }

        if (options.showSecrets) {
            __flags.push('--show-secrets');
        }

        if (options.stack != null) {
            __flags.push('--stack', '' + options.stack);
        }

        if (options.suppressOutputs) {
            __flags.push('--suppress-outputs');
        }

        if (options.suppressPermalink != null) {
            __flags.push('--suppress-permalink', '' + options.suppressPermalink);
        }

        if (options.suppressProgress) {
            __flags.push('--suppress-progress');
        }

        for (const __item of options.target ?? []) {
            if (__item != null) {
                __flags.push('--target', '' + __item);
            }

        }

        if (options.targetDependents) {
            __flags.push('--target-dependents');
        }

        for (const __item of options.targetReplace ?? []) {
            if (__item != null) {
                __flags.push('--target-replace', '' + __item);
            }

        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    projectLs(options: PulumiProjectLsOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('project');
        __final.push('ls');

        const __flags: string[] = [];

        if (options.json) {
            __flags.push('--json');
        }

        if (options.organization != null) {
            __flags.push('--organization', '' + options.organization);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    refresh(options: PulumiRefreshOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('refresh');

        const __flags: string[] = [];

        if (options.clearPendingCreates) {
            __flags.push('--clear-pending-creates');
        }

        if (options.client != null) {
            __flags.push('--client', '' + options.client);
        }

        for (const __item of options.config ?? []) {
            if (__item != null) {
                __flags.push('--config', '' + __item);
            }

        }

        if (options.configFile != null) {
            __flags.push('--config-file', '' + options.configFile);
        }

        if (options.configPath) {
            __flags.push('--config-path');
        }

        if (options.copilot) {
            __flags.push('--copilot');
        }

        if (options.debug) {
            __flags.push('--debug');
        }

        if (options.diff) {
            __flags.push('--diff');
        }

        for (const __item of options.exclude ?? []) {
            if (__item != null) {
                __flags.push('--exclude', '' + __item);
            }

        }

        if (options.excludeDependents) {
            __flags.push('--exclude-dependents');
        }

        if (options.execAgent != null) {
            __flags.push('--exec-agent', '' + options.execAgent);
        }

        if (options.execKind != null) {
            __flags.push('--exec-kind', '' + options.execKind);
        }

        if (options.expectNoChanges) {
            __flags.push('--expect-no-changes');
        }

        for (const __item of options.importPendingCreates ?? []) {
            if (__item != null) {
                __flags.push('--import-pending-creates', '' + __item);
            }

        }

        if (options.json) {
            __flags.push('--json');
        }

        if (options.message != null) {
            __flags.push('--message', '' + options.message);
        }

        if (options.neo) {
            __flags.push('--neo');
        }

        if (options.parallel != null) {
            __flags.push('--parallel', '' + options.parallel);
        }

        if (options.previewOnly) {
            __flags.push('--preview-only');
        }

        if (options.runProgram) {
            __flags.push('--run-program');
        }

        if (options.showReplacementSteps) {
            __flags.push('--show-replacement-steps');
        }

        if (options.showSames) {
            __flags.push('--show-sames');
        }

        if (options.skipPendingCreates) {
            __flags.push('--skip-pending-creates');
        }

        if (options.skipPreview) {
            __flags.push('--skip-preview');
        }

        if (options.stack != null) {
            __flags.push('--stack', '' + options.stack);
        }

        if (options.suppressOutputs) {
            __flags.push('--suppress-outputs');
        }

        if (options.suppressPermalink != null) {
            __flags.push('--suppress-permalink', '' + options.suppressPermalink);
        }

        if (options.suppressProgress) {
            __flags.push('--suppress-progress');
        }

        for (const __item of options.target ?? []) {
            if (__item != null) {
                __flags.push('--target', '' + __item);
            }

        }

        if (options.targetDependents) {
            __flags.push('--target-dependents');
        }

        if (options.yes) {
            __flags.push('--yes');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    replayEvents(options: PulumiReplayEventsOptions, kind: string, eventsFile: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('replay-events');

        const __flags: string[] = [];

        if (options.debug) {
            __flags.push('--debug');
        }

        if (options.delay != null) {
            __flags.push('--delay', '' + options.delay);
        }

        if (options.diff) {
            __flags.push('--diff');
        }

        if (options.json) {
            __flags.push('--json');
        }

        if (options.period != null) {
            __flags.push('--period', '' + options.period);
        }

        if (options.preview) {
            __flags.push('--preview');
        }

        if (options.showConfig) {
            __flags.push('--show-config');
        }

        if (options.showReads) {
            __flags.push('--show-reads');
        }

        if (options.showReplacementSteps) {
            __flags.push('--show-replacement-steps');
        }

        if (options.showSames) {
            __flags.push('--show-sames');
        }

        if (options.suppressOutputs) {
            __flags.push('--suppress-outputs');
        }

        if (options.suppressProgress) {
            __flags.push('--suppress-progress');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        __arguments.push('' + kind);

        __arguments.push('' + eventsFile);

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    schemaCheck(options: PulumiSchemaCheckOptions, schemaSource: string, ...providerParameter: string[]): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('schema');
        __final.push('check');

        const __flags: string[] = [];

        if (options.allowDanglingReferences) {
            __flags.push('--allow-dangling-references');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        for (const __item of schemaSource ?? []) {
            __arguments.push('' + __item);

        }

        if (providerParameter != null) {
            for (const __item of providerParameter ?? []) {
                __arguments.push('' + __item);

            }

        }
        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    stackChangeSecretsProvider(options: PulumiStackChangeSecretsProviderOptions, newSecretsProvider: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('stack');
        __final.push('change-secrets-provider');

        const __flags: string[] = [];

        if (options.stack != null) {
            __flags.push('--stack', '' + options.stack);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        __arguments.push('' + newSecretsProvider);

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    stackExport(options: PulumiStackExportOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('stack');
        __final.push('export');

        const __flags: string[] = [];

        if (options.file != null) {
            __flags.push('--file', '' + options.file);
        }

        if (options.showSecrets) {
            __flags.push('--show-secrets');
        }

        if (options.stack != null) {
            __flags.push('--stack', '' + options.stack);
        }

        if (options.version != null) {
            __flags.push('--version', '' + options.version);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    stackGraph(options: PulumiStackGraphOptions, filename: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('stack');
        __final.push('graph');

        const __flags: string[] = [];

        if (options.dependencyEdgeColor != null) {
            __flags.push('--dependency-edge-color', '' + options.dependencyEdgeColor);
        }

        if (options.dotFragment != null) {
            __flags.push('--dot-fragment', '' + options.dotFragment);
        }

        if (options.ignoreDependencyEdges) {
            __flags.push('--ignore-dependency-edges');
        }

        if (options.ignoreParentEdges) {
            __flags.push('--ignore-parent-edges');
        }

        if (options.parentEdgeColor != null) {
            __flags.push('--parent-edge-color', '' + options.parentEdgeColor);
        }

        if (options.shortNodeName) {
            __flags.push('--short-node-name');
        }

        if (options.stack != null) {
            __flags.push('--stack', '' + options.stack);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        __arguments.push('' + filename);

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    stackHistory(options: PulumiStackHistoryOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('stack');
        __final.push('history');

        const __flags: string[] = [];

        if (options.fullDates) {
            __flags.push('--full-dates');
        }

        if (options.json) {
            __flags.push('--json');
        }

        if (options.page != null) {
            __flags.push('--page', '' + options.page);
        }

        if (options.pageSize != null) {
            __flags.push('--page-size', '' + options.pageSize);
        }

        if (options.showSecrets) {
            __flags.push('--show-secrets');
        }

        if (options.stack != null) {
            __flags.push('--stack', '' + options.stack);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    stackImport(options: PulumiStackImportOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('stack');
        __final.push('import');

        const __flags: string[] = [];

        if (options.file != null) {
            __flags.push('--file', '' + options.file);
        }

        if (options.force) {
            __flags.push('--force');
        }

        if (options.stack != null) {
            __flags.push('--stack', '' + options.stack);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    stackInit(options: PulumiStackInitOptions, stackName?: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('stack');
        __final.push('init');

        const __flags: string[] = [];

        if (options.copyConfigFrom != null) {
            __flags.push('--copy-config-from', '' + options.copyConfigFrom);
        }

        if (options.noSelect) {
            __flags.push('--no-select');
        }

        if (options.remoteConfig) {
            __flags.push('--remote-config');
        }

        if (options.secretsProvider != null) {
            __flags.push('--secrets-provider', '' + options.secretsProvider);
        }

        if (options.stack != null) {
            __flags.push('--stack', '' + options.stack);
        }

        for (const __item of options.teams ?? []) {
            if (__item != null) {
                __flags.push('--teams', '' + __item);
            }

        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (stackName != null) {
            __arguments.push('' + stackName);

        }
        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    stackLs(options: PulumiStackLsOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('stack');
        __final.push('ls');

        const __flags: string[] = [];

        if (options.all) {
            __flags.push('--all');
        }

        if (options.json) {
            __flags.push('--json');
        }

        if (options.organization != null) {
            __flags.push('--organization', '' + options.organization);
        }

        if (options.project != null) {
            __flags.push('--project', '' + options.project);
        }

        if (options.tag != null) {
            __flags.push('--tag', '' + options.tag);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    stackOutput(options: PulumiStackOutputOptions, propertyName?: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('stack');
        __final.push('output');

        const __flags: string[] = [];

        if (options.json) {
            __flags.push('--json');
        }

        if (options.shell) {
            __flags.push('--shell');
        }

        if (options.showSecrets) {
            __flags.push('--show-secrets');
        }

        if (options.stack != null) {
            __flags.push('--stack', '' + options.stack);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (propertyName != null) {
            __arguments.push('' + propertyName);

        }
        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    stackRename(options: PulumiStackRenameOptions, newStackName: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('stack');
        __final.push('rename');

        const __flags: string[] = [];

        if (options.stack != null) {
            __flags.push('--stack', '' + options.stack);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        __arguments.push('' + newStackName);

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    stackRm(options: PulumiStackRmOptions, stackName?: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('stack');
        __final.push('rm');

        const __flags: string[] = [];

        if (options.force) {
            __flags.push('--force');
        }

        if (options.preserveConfig) {
            __flags.push('--preserve-config');
        }

        if (options.removeBackups) {
            __flags.push('--remove-backups');
        }

        if (options.stack != null) {
            __flags.push('--stack', '' + options.stack);
        }

        if (options.yes) {
            __flags.push('--yes');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (stackName != null) {
            __arguments.push('' + stackName);

        }
        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    stackSelect(options: PulumiStackSelectOptions, stack?: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('stack');
        __final.push('select');

        const __flags: string[] = [];

        if (options.create) {
            __flags.push('--create');
        }

        if (options.secretsProvider != null) {
            __flags.push('--secrets-provider', '' + options.secretsProvider);
        }

        if (options.stack != null) {
            __flags.push('--stack', '' + options.stack);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (stack != null) {
            __arguments.push('' + stack);

        }
        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    stackTagGet(options: PulumiStackTagGetOptions, name: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('stack');
        __final.push('tag');
        __final.push('get');

        const __flags: string[] = [];

        __final.push(... __flags);

        const __arguments: string[] = [];

        __arguments.push('' + name);

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    stackTagLs(options: PulumiStackTagLsOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('stack');
        __final.push('tag');
        __final.push('ls');

        const __flags: string[] = [];

        if (options.json) {
            __flags.push('--json');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    stackTagRm(options: PulumiStackTagRmOptions, name: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('stack');
        __final.push('tag');
        __final.push('rm');

        const __flags: string[] = [];

        __final.push(... __flags);

        const __arguments: string[] = [];

        __arguments.push('' + name);

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    stackTagSet(options: PulumiStackTagSetOptions, name: string, value: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('stack');
        __final.push('tag');
        __final.push('set');

        const __flags: string[] = [];

        __final.push(... __flags);

        const __arguments: string[] = [];

        __arguments.push('' + name);

        __arguments.push('' + value);

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    stackUnselect(options: PulumiStackUnselectOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('stack');
        __final.push('unselect');

        const __flags: string[] = [];

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    stack(options: PulumiStackOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('stack');

        const __flags: string[] = [];

        if (options.showIds) {
            __flags.push('--show-ids');
        }

        if (options.showName) {
            __flags.push('--show-name');
        }

        if (options.showSecrets) {
            __flags.push('--show-secrets');
        }

        if (options.showUrns) {
            __flags.push('--show-urns');
        }

        if (options.stack != null) {
            __flags.push('--stack', '' + options.stack);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    stateDelete(options: PulumiStateDeleteOptions, resourceUrn?: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('state');
        __final.push('delete');

        const __flags: string[] = [];

        if (options.all) {
            __flags.push('--all');
        }

        if (options.force) {
            __flags.push('--force');
        }

        if (options.stack != null) {
            __flags.push('--stack', '' + options.stack);
        }

        if (options.targetDependents) {
            __flags.push('--target-dependents');
        }

        if (options.yes) {
            __flags.push('--yes');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (resourceUrn != null) {
            __arguments.push('' + resourceUrn);

        }
        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    stateEdit(options: PulumiStateEditOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('state');
        __final.push('edit');

        const __flags: string[] = [];

        if (options.stack != null) {
            __flags.push('--stack', '' + options.stack);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    stateMove(options: PulumiStateMoveOptions, ...urn: string[]): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('state');
        __final.push('move');

        const __flags: string[] = [];

        if (options.dest != null) {
            __flags.push('--dest', '' + options.dest);
        }

        if (options.includeParents) {
            __flags.push('--include-parents');
        }

        if (options.source != null) {
            __flags.push('--source', '' + options.source);
        }

        if (options.yes) {
            __flags.push('--yes');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        for (const __item of urn ?? []) {
            __arguments.push('' + __item);

        }

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    stateProtect(options: PulumiStateProtectOptions, ...resourceUrn: string[]): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('state');
        __final.push('protect');

        const __flags: string[] = [];

        if (options.all) {
            __flags.push('--all');
        }

        if (options.stack != null) {
            __flags.push('--stack', '' + options.stack);
        }

        if (options.yes) {
            __flags.push('--yes');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (resourceUrn != null) {
            for (const __item of resourceUrn ?? []) {
                __arguments.push('' + __item);

            }

        }
        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    stateRename(options: PulumiStateRenameOptions, resourceUrn?: string, newName?: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('state');
        __final.push('rename');

        const __flags: string[] = [];

        if (options.stack != null) {
            __flags.push('--stack', '' + options.stack);
        }

        if (options.yes) {
            __flags.push('--yes');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (resourceUrn != null) {
            __arguments.push('' + resourceUrn);

        }
        if (newName != null) {
            __arguments.push('' + newName);

        }
        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    stateRepair(options: PulumiStateRepairOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('state');
        __final.push('repair');

        const __flags: string[] = [];

        if (options.stack != null) {
            __flags.push('--stack', '' + options.stack);
        }

        if (options.yes) {
            __flags.push('--yes');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    stateTaint(options: PulumiStateTaintOptions, ...resourceUrn: string[]): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('state');
        __final.push('taint');

        const __flags: string[] = [];

        if (options.stack != null) {
            __flags.push('--stack', '' + options.stack);
        }

        if (options.yes) {
            __flags.push('--yes');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (resourceUrn != null) {
            for (const __item of resourceUrn ?? []) {
                __arguments.push('' + __item);

            }

        }
        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    stateUnprotect(options: PulumiStateUnprotectOptions, ...resourceUrn: string[]): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('state');
        __final.push('unprotect');

        const __flags: string[] = [];

        if (options.all) {
            __flags.push('--all');
        }

        if (options.stack != null) {
            __flags.push('--stack', '' + options.stack);
        }

        if (options.yes) {
            __flags.push('--yes');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (resourceUrn != null) {
            for (const __item of resourceUrn ?? []) {
                __arguments.push('' + __item);

            }

        }
        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    stateUntaint(options: PulumiStateUntaintOptions, ...resourceUrn: string[]): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('state');
        __final.push('untaint');

        const __flags: string[] = [];

        if (options.all) {
            __flags.push('--all');
        }

        if (options.stack != null) {
            __flags.push('--stack', '' + options.stack);
        }

        if (options.yes) {
            __flags.push('--yes');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (resourceUrn != null) {
            for (const __item of resourceUrn ?? []) {
                __arguments.push('' + __item);

            }

        }
        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    stateUpgrade(options: PulumiStateUpgradeOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('state');
        __final.push('upgrade');

        const __flags: string[] = [];

        if (options.yes) {
            __flags.push('--yes');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    templatePublish(options: PulumiTemplatePublishOptions, directory: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('template');
        __final.push('publish');

        const __flags: string[] = [];

        __flags.push('--name', '' + options.name);

        if (options.publisher != null) {
            __flags.push('--publisher', '' + options.publisher);
        }

        __flags.push('--version', '' + options.version);

        __final.push(... __flags);

        const __arguments: string[] = [];

        __arguments.push('' + directory);

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    up(options: PulumiUpOptions, templateOrUrl?: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('up');

        const __flags: string[] = [];

        for (const __item of options.attachDebugger ?? []) {
            if (__item != null) {
                __flags.push('--attach-debugger', '' + __item);
            }

        }

        if (options.client != null) {
            __flags.push('--client', '' + options.client);
        }

        for (const __item of options.config ?? []) {
            if (__item != null) {
                __flags.push('--config', '' + __item);
            }

        }

        if (options.configFile != null) {
            __flags.push('--config-file', '' + options.configFile);
        }

        if (options.configPath) {
            __flags.push('--config-path');
        }

        if (options.continueOnError) {
            __flags.push('--continue-on-error');
        }

        if (options.copilot) {
            __flags.push('--copilot');
        }

        if (options.debug) {
            __flags.push('--debug');
        }

        if (options.diff) {
            __flags.push('--diff');
        }

        for (const __item of options.exclude ?? []) {
            if (__item != null) {
                __flags.push('--exclude', '' + __item);
            }

        }

        if (options.excludeDependents) {
            __flags.push('--exclude-dependents');
        }

        if (options.execAgent != null) {
            __flags.push('--exec-agent', '' + options.execAgent);
        }

        if (options.execKind != null) {
            __flags.push('--exec-kind', '' + options.execKind);
        }

        if (options.expectNoChanges) {
            __flags.push('--expect-no-changes');
        }

        if (options.json) {
            __flags.push('--json');
        }

        if (options.message != null) {
            __flags.push('--message', '' + options.message);
        }

        if (options.neo) {
            __flags.push('--neo');
        }

        if (options.neoTaskOnFailure) {
            __flags.push('--neo-task-on-failure');
        }

        if (options.parallel != null) {
            __flags.push('--parallel', '' + options.parallel);
        }

        if (options.plan != null) {
            __flags.push('--plan', '' + options.plan);
        }

        for (const __item of options.policyPack ?? []) {
            if (__item != null) {
                __flags.push('--policy-pack', '' + __item);
            }

        }

        for (const __item of options.policyPackConfig ?? []) {
            if (__item != null) {
                __flags.push('--policy-pack-config', '' + __item);
            }

        }

        if (options.refresh != null) {
            __flags.push('--refresh', '' + options.refresh);
        }

        for (const __item of options.replace ?? []) {
            if (__item != null) {
                __flags.push('--replace', '' + __item);
            }

        }

        if (options.runProgram) {
            __flags.push('--run-program');
        }

        if (options.secretsProvider != null) {
            __flags.push('--secrets-provider', '' + options.secretsProvider);
        }

        if (options.showConfig) {
            __flags.push('--show-config');
        }

        if (options.showFullOutput) {
            __flags.push('--show-full-output');
        }

        if (options.showPolicyRemediations) {
            __flags.push('--show-policy-remediations');
        }

        if (options.showReads) {
            __flags.push('--show-reads');
        }

        if (options.showReplacementSteps) {
            __flags.push('--show-replacement-steps');
        }

        if (options.showSames) {
            __flags.push('--show-sames');
        }

        if (options.showSecrets) {
            __flags.push('--show-secrets');
        }

        if (options.skipPreview) {
            __flags.push('--skip-preview');
        }

        if (options.stack != null) {
            __flags.push('--stack', '' + options.stack);
        }

        if (options.strict) {
            __flags.push('--strict');
        }

        if (options.suppressOutputs) {
            __flags.push('--suppress-outputs');
        }

        if (options.suppressPermalink != null) {
            __flags.push('--suppress-permalink', '' + options.suppressPermalink);
        }

        if (options.suppressProgress) {
            __flags.push('--suppress-progress');
        }

        for (const __item of options.target ?? []) {
            if (__item != null) {
                __flags.push('--target', '' + __item);
            }

        }

        if (options.targetDependents) {
            __flags.push('--target-dependents');
        }

        for (const __item of options.targetReplace ?? []) {
            if (__item != null) {
                __flags.push('--target-replace', '' + __item);
            }

        }

        if (options.yes) {
            __flags.push('--yes');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (templateOrUrl != null) {
            __arguments.push('' + templateOrUrl);

        }
        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    version(options: PulumiVersionOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('version');

        const __flags: string[] = [];

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    viewTrace(options: PulumiViewTraceOptions, traceFile: string): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('view-trace');

        const __flags: string[] = [];

        if (options.port != null) {
            __flags.push('--port', '' + options.port);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        __arguments.push('' + traceFile);

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    watch(options: PulumiWatchOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('watch');

        const __flags: string[] = [];

        for (const __item of options.config ?? []) {
            if (__item != null) {
                __flags.push('--config', '' + __item);
            }

        }

        if (options.configFile != null) {
            __flags.push('--config-file', '' + options.configFile);
        }

        if (options.configPath) {
            __flags.push('--config-path');
        }

        if (options.debug) {
            __flags.push('--debug');
        }

        if (options.execKind != null) {
            __flags.push('--exec-kind', '' + options.execKind);
        }

        if (options.message != null) {
            __flags.push('--message', '' + options.message);
        }

        if (options.parallel != null) {
            __flags.push('--parallel', '' + options.parallel);
        }

        for (const __item of options.path ?? []) {
            if (__item != null) {
                __flags.push('--path', '' + __item);
            }

        }

        for (const __item of options.policyPack ?? []) {
            if (__item != null) {
                __flags.push('--policy-pack', '' + __item);
            }

        }

        for (const __item of options.policyPackConfig ?? []) {
            if (__item != null) {
                __flags.push('--policy-pack-config', '' + __item);
            }

        }

        if (options.refresh) {
            __flags.push('--refresh');
        }

        if (options.secretsProvider != null) {
            __flags.push('--secrets-provider', '' + options.secretsProvider);
        }

        if (options.showConfig) {
            __flags.push('--show-config');
        }

        if (options.showReplacementSteps) {
            __flags.push('--show-replacement-steps');
        }

        if (options.showSames) {
            __flags.push('--show-sames');
        }

        if (options.stack != null) {
            __flags.push('--stack', '' + options.stack);
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }

    whoami(options: PulumiWhoamiOptions): ReturnType<API['__run']> {
        const __final: string[] = [];
        __final.push('whoami');

        const __flags: string[] = [];

        if (options.json) {
            __flags.push('--json');
        }

        if (options.verbose) {
            __flags.push('--verbose');
        }

        __final.push(... __flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push('--')
            __final.push(... __arguments)
        }

        return this.__run(options, __final);
    }
}

/** Options for the `pulumi ` command. */
export interface PulumiOptions extends BaseOptions {
    /** Colorize output. Choices are: always, never, raw, auto */
    color?: string;
    /** Run pulumi as if it had been started in another directory */
    cwd?: string;
    /** Disable integrity checking of checkpoint files */
    disableIntegrityChecking?: boolean;
    /** Enable emojis in the output */
    emoji?: boolean;
    /** Show fully-qualified stack names */
    fullyQualifyStackNames?: boolean;
    /** Flow log settings to child processes (like plugins) */
    logflow?: boolean;
    /** Log to stderr instead of to files */
    logtostderr?: boolean;
    /** Enable more precise (and expensive) memory allocation profiles by setting runtime.MemProfileRate */
    memprofilerate?: number;
    /** Disable interactive mode for all commands */
    nonInteractive?: boolean;
    /** Export OpenTelemetry data to the specified endpoint. Use file:// for local JSON files, grpc:// for remote collectors */
    otel?: string;
    /** Emit CPU and memory profiles and an execution trace to '[filename].[pid].{cpu,mem,trace}', respectively */
    profiling?: string;
    /** Emit tracing to the specified endpoint. Use the `file:` scheme to write tracing data to a local file */
    tracing?: string;
    /** Include the tracing header with the given contents. */
    tracingHeader?: string;
    /** Enable verbose logging (e.g., v=3); anything >3 is very verbose */
    verbose?: number;
}

/** Options for the `pulumi about` command. */
export interface PulumiAboutOptions extends BaseOptions {
    /** Emit output as JSON */
    json?: boolean;
    /** The name of the stack to get info on. Defaults to the current stack */
    stack?: string;
    /** Include transitive dependencies */
    transitive?: boolean;
}

/** Options for the `pulumi about env` command. */
export interface PulumiAboutEnvOptions extends BaseOptions {
}

/** Options for the `pulumi ai` command. */
export interface PulumiAiOptions extends BaseOptions {
}

/** Options for the `pulumi ai web` command. */
export interface PulumiAiWebOptions extends BaseOptions {
    /** Language to use for the prompt - this defaults to TypeScript. [TypeScript, Python, Go, C#, Java, YAML] */
    language?: string;
    /** Opt-out of automatically submitting the prompt to Pulumi AI */
    noAutoSubmit?: boolean;
}

/** Options for the `pulumi cancel` command. */
export interface PulumiCancelOptions extends BaseOptions {
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Skip confirmation prompts, and proceed with cancellation anyway */
    yes?: boolean;
}

/** Options for the `pulumi config` command. */
export interface PulumiConfigOptions extends BaseOptions {
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

/** Options for the `pulumi config cp` command. */
export interface PulumiConfigCpOptions extends BaseOptions {
    /** The name of the new stack to copy the config to */
    dest?: string;
    /** The key contains a path to a property in a map or list to set */
    path?: boolean;
}

/** Options for the `pulumi config env` command. */
export interface PulumiConfigEnvOptions extends BaseOptions {
}

/** Options for the `pulumi config env add` command. */
export interface PulumiConfigEnvAddOptions extends BaseOptions {
    /** Show secret values in plaintext instead of ciphertext */
    showSecrets?: boolean;
    /** True to save changes without prompting */
    yes?: boolean;
}

/** Options for the `pulumi config env init` command. */
export interface PulumiConfigEnvInitOptions extends BaseOptions {
    /** The name of the environment to create. Defaults to "<project name>/<stack name>" */
    env?: string;
    /** Do not remove configuration values from the stack after creating the environment */
    keepConfig?: boolean;
    /** Show secret values in plaintext instead of ciphertext */
    showSecrets?: boolean;
    /** True to save the created environment without prompting */
    yes?: boolean;
}

/** Options for the `pulumi config env ls` command. */
export interface PulumiConfigEnvLsOptions extends BaseOptions {
    /** Emit output as JSON */
    json?: boolean;
}

/** Options for the `pulumi config env rm` command. */
export interface PulumiConfigEnvRmOptions extends BaseOptions {
    /** Show secret values in plaintext instead of ciphertext */
    showSecrets?: boolean;
    /** True to save changes without prompting */
    yes?: boolean;
}

/** Options for the `pulumi config get` command. */
export interface PulumiConfigGetOptions extends BaseOptions {
    /** Emit output as JSON */
    json?: boolean;
    /** Open and resolve any environments listed in the stack configuration */
    open?: boolean;
    /** The key contains a path to a property in a map or list to get */
    path?: boolean;
}

/** Options for the `pulumi config refresh` command. */
export interface PulumiConfigRefreshOptions extends BaseOptions {
    /** Overwrite configuration file, if it exists, without creating a backup */
    force?: boolean;
}

/** Options for the `pulumi config rm` command. */
export interface PulumiConfigRmOptions extends BaseOptions {
    /** The key contains a path to a property in a map or list to remove */
    path?: boolean;
}

/** Options for the `pulumi config rm-all` command. */
export interface PulumiConfigRmAllOptions extends BaseOptions {
    /** Parse the keys as paths in a map or list rather than raw strings */
    path?: boolean;
}

/** Options for the `pulumi config set` command. */
export interface PulumiConfigSetOptions extends BaseOptions {
    /** The key contains a path to a property in a map or list to set */
    path?: boolean;
    /** Save the value as plaintext (unencrypted) */
    plaintext?: boolean;
    /** Encrypt the value instead of storing it in plaintext */
    secret?: boolean;
    /** Save the value as the given type.  Allowed values are string, bool, int, and float */
    type?: string;
}

/** Options for the `pulumi config set-all` command. */
export interface PulumiConfigSetAllOptions extends BaseOptions {
    /** Read values from a JSON string in the format produced by 'pulumi config --json' */
    json?: string;
    /** Parse the keys as paths in a map or list rather than raw strings */
    path?: boolean;
    /** Marks a value as plaintext (unencrypted) */
    plaintext?: string[];
    /** Marks a value as secret to be encrypted */
    secret?: string[];
}

/** Options for the `pulumi console` command. */
export interface PulumiConsoleOptions extends BaseOptions {
    /** The name of the stack to view */
    stack?: string;
}

/** Options for the `pulumi convert` command. */
export interface PulumiConvertOptions extends BaseOptions {
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

/** Options for the `pulumi convert-trace` command. */
export interface PulumiConvertTraceOptions extends BaseOptions {
    /** the sample granularity */
    granularity?: string;
    /** true to ignore log spans */
    ignoreLogSpans?: boolean;
    /** true to export to OpenTelemetry */
    otel?: boolean;
}

/** Options for the `pulumi deployment` command. */
export interface PulumiDeploymentOptions extends BaseOptions {
    /** Override the file name where the deployment settings are specified. Default is Pulumi.[stack].deploy.yaml */
    configFile?: string;
}

/** Options for the `pulumi deployment run` command. */
export interface PulumiDeploymentRunOptions extends BaseOptions {
    /** The agent pool to use to run the deployment job. When empty, the Pulumi Cloud shared queue will be used. */
    agentPoolId?: string;
    /** Environment variables to use in the remote operation of the form NAME=value (e.g. `--env FOO=bar`) */
    env?: string[];
    /** Environment variables with secret values to use in the remote operation of the form NAME=secretvalue (e.g. `--env FOO=secret`) */
    envSecret?: string[];
    /** The Docker image to use for the executor */
    executorImage?: string;
    /** The password for the credentials with access to the Docker image to use for the executor */
    executorImagePassword?: string;
    /** The username for the credentials with access to the Docker image to use for the executor */
    executorImageUsername?: string;
    /** Git personal access token */
    gitAuthAccessToken?: string;
    /** Git password; for use with username or with an SSH private key */
    gitAuthPassword?: string;
    /** Git SSH private key; use --git-auth-password for the password, if needed */
    gitAuthSshPrivateKey?: string;
    /** Git SSH private key path; use --git-auth-password for the password, if needed */
    gitAuthSshPrivateKeyPath?: string;
    /** Git username */
    gitAuthUsername?: string;
    /** Git branch to deploy; this is mutually exclusive with --git-commit; either value needs to be specified */
    gitBranch?: string;
    /** Git commit hash of the commit to deploy (if used, HEAD will be in detached mode); this is mutually exclusive with --git-branch; either value needs to be specified */
    gitCommit?: string;
    /** The directory to work from in the project's source repository where Pulumi.yaml is located; used when Pulumi.yaml is not in the project source root */
    gitRepoDir?: string;
    /** Inherit deployment settings from the current stack */
    inheritSettings?: boolean;
    /** Commands to run before the remote operation */
    preRunCommand?: string[];
    /** Whether to skip the default dependency installation step */
    skipInstallDependencies?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Suppress display of the state permalink */
    suppressPermalink?: boolean;
    /** Suppress log streaming of the deployment job */
    suppressStreamLogs?: boolean;
}

/** Options for the `pulumi deployment settings` command. */
export interface PulumiDeploymentSettingsOptions extends BaseOptions {
}

/** Options for the `pulumi deployment settings configure` command. */
export interface PulumiDeploymentSettingsConfigureOptions extends BaseOptions {
    /** Git SSH private key */
    gitAuthSshPrivateKey?: string;
    /** Private key path */
    gitAuthSshPrivateKeyPath?: string;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi deployment settings destroy` command. */
export interface PulumiDeploymentSettingsDestroyOptions extends BaseOptions {
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Automatically confirm every confirmation prompt */
    yes?: boolean;
}

/** Options for the `pulumi deployment settings env` command. */
export interface PulumiDeploymentSettingsEnvOptions extends BaseOptions {
    /** whether the key should be removed */
    remove?: boolean;
    /** whether the value should be treated as a secret and be encrypted */
    secret?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi deployment settings init` command. */
export interface PulumiDeploymentSettingsInitOptions extends BaseOptions {
    /** Forces content to be generated even if it is already configured */
    force?: boolean;
    /** Git SSH private key */
    gitAuthSshPrivateKey?: string;
    /** Git SSH private key path */
    gitAuthSshPrivateKeyPath?: string;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi deployment settings pull` command. */
export interface PulumiDeploymentSettingsPullOptions extends BaseOptions {
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi deployment settings push` command. */
export interface PulumiDeploymentSettingsPushOptions extends BaseOptions {
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Automatically confirm every confirmation prompt */
    yes?: boolean;
}

/** Options for the `pulumi destroy` command. */
export interface PulumiDestroyOptions extends BaseOptions {
    /** The address of an existing language runtime host to connect to */
    client?: string;
    /** Config to use during the destroy and save to the stack config file */
    config?: string[];
    /** Use the configuration values in the specified file rather than detecting the file name */
    configFile?: string;
    /** Config keys contain a path to a property in a map or list to set */
    configPath?: boolean;
    /** Continue to perform the destroy operation despite the occurrence of errors (can also be set with PULUMI_CONTINUE_ON_ERROR env var) */
    continueOnError?: boolean;
    /** [DEPRECATED] Use --neo instead. Enable Pulumi Neo's assistance for improved CLI experience and insights (can also be set with PULUMI_COPILOT environment variable) */
    copilot?: boolean;
    /** Print detailed debugging output during resource operations */
    debug?: boolean;
    /** Display operation as a rich diff showing the overall change */
    diff?: boolean;
    /** Specify a resource URN to ignore. These resources will not be updated. Multiple resources can be specified using --exclude urn1 --exclude urn2. Wildcards (*, **) are also supported */
    exclude?: string[];
    /** Do not destroy protected resources. Destroy all other resources. */
    excludeProtected?: boolean;
    execAgent?: string;
    execKind?: string;
    /** Serialize the destroy diffs, operations, and overall output as JSON */
    json?: boolean;
    /** Optional message to associate with the destroy operation */
    message?: string;
    /** Enable Pulumi Neo's assistance for improved CLI experience and insights (can also be set with PULUMI_NEO environment variable) */
    neo?: boolean;
    /** Allow P resource operations to run in parallel at once (1 for no parallelism). */
    parallel?: number;
    /** Only show a preview of the destroy, but don't perform the destroy itself */
    previewOnly?: boolean;
    /** Refresh the state of the stack's resources before this update */
    refresh?: string;
    /** Remove the stack and its config file after all resources in the stack have been deleted */
    remove?: boolean;
    /** Run the program to determine up-to-date state for providers to destroy resources */
    runProgram?: boolean;
    /** Show configuration keys and variables */
    showConfig?: boolean;
    /** Display full length of inputs & outputs */
    showFullOutput?: boolean;
    /** Show detailed resource replacement creates and deletes instead of a single step */
    showReplacementSteps?: boolean;
    /** Show resources that don't need to be updated because they haven't changed, alongside those that do */
    showSames?: boolean;
    /** Do not calculate a preview before performing the destroy */
    skipPreview?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Suppress display of stack outputs (in case they contain sensitive values) */
    suppressOutputs?: boolean;
    /** Suppress display of the state permalink */
    suppressPermalink?: string;
    /** Suppress display of periodic progress dots */
    suppressProgress?: boolean;
    /** Specify a single resource URN to destroy. All resources necessary to destroy this target will also be destroyed. Multiple resources can be specified using: --target urn1 --target urn2. Wildcards (*, **) are also supported */
    target?: string[];
    /** Allows destroying of dependent targets discovered but not specified in --target list */
    targetDependents?: boolean;
    /** Automatically approve and perform the destroy after previewing it */
    yes?: boolean;
}

/** Options for the `pulumi env` command. */
export interface PulumiEnvOptions extends BaseOptions {
    /** The name of the environment to operate on. */
    env?: string;
}

/** Options for the `pulumi env clone` command. */
export interface PulumiEnvCloneOptions extends BaseOptions {
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
    /** the output format to use. May be 'dotenv', 'json', 'yaml', 'detailed', or 'shell' */
    format?: string;
    /** Show the diff for a specific path */
    path?: string;
    /** Show static secrets in plaintext rather than ciphertext */
    showSecrets?: boolean;
}

/** Options for the `pulumi env edit` command. */
export interface PulumiEnvEditOptions extends BaseOptions {
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
    /** Set to print just the definition. */
    definition?: boolean;
    /** Show static secrets in plaintext rather than ciphertext */
    showSecrets?: boolean;
    /** Set to print just the value in the given format. May be 'dotenv', 'json', 'detailed', 'shell' or 'string' */
    value?: string;
}

/** Options for the `pulumi env init` command. */
export interface PulumiEnvInitOptions extends BaseOptions {
    /** the file to use to initialize the environment, if any. Pass `-` to read from standard input. */
    file?: string;
}

/** Options for the `pulumi env ls` command. */
export interface PulumiEnvLsOptions extends BaseOptions {
    /** Filter returned environments to those in a specific organization */
    organization?: string;
    /** Filter returned environments to those in a specific project */
    project?: string;
}

/** Options for the `pulumi env open` command. */
export interface PulumiEnvOpenOptions extends BaseOptions {
    /** open an environment draft with --draft=<change-request-id> */
    draft?: string;
    /** the output format to use. May be 'dotenv', 'json', 'yaml', 'detailed', 'shell' or 'string' */
    format?: string;
    /** the lifetime of the opened environment in the form HhMm (e.g. 2h, 1h30m, 15m) */
    lifetime?: string;
}

/** Options for the `pulumi env rm` command. */
export interface PulumiEnvRmOptions extends BaseOptions {
    /** Skip confirmation prompts, and proceed with removal anyway */
    yes?: boolean;
}

/** Options for the `pulumi env rotate` command. */
export interface PulumiEnvRotateOptions extends BaseOptions {
}

/** Options for the `pulumi env run` command. */
export interface PulumiEnvRunOptions extends BaseOptions {
    /** open an environment draft with --draft=<change-request-id> */
    draft?: string;
    /** true to treat the command as interactive and disable output filters */
    interactive?: boolean;
    /** the lifetime of the opened environment */
    lifetime?: string;
}

/** Options for the `pulumi env set` command. */
export interface PulumiEnvSetOptions extends BaseOptions {
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

/** Options for the `pulumi env tag` command. */
export interface PulumiEnvTagOptions extends BaseOptions {
    /** display times in UTC */
    utc?: boolean;
}

/** Options for the `pulumi env tag get` command. */
export interface PulumiEnvTagGetOptions extends BaseOptions {
    /** display times in UTC */
    utc?: boolean;
}

/** Options for the `pulumi env tag ls` command. */
export interface PulumiEnvTagLsOptions extends BaseOptions {
    /** the command to use to page through the environment's version tags */
    pager?: string;
    /** display times in UTC */
    utc?: boolean;
}

/** Options for the `pulumi env tag mv` command. */
export interface PulumiEnvTagMvOptions extends BaseOptions {
    /** display times in UTC */
    utc?: boolean;
}

/** Options for the `pulumi env tag rm` command. */
export interface PulumiEnvTagRmOptions extends BaseOptions {
}

/** Options for the `pulumi env version` command. */
export interface PulumiEnvVersionOptions extends BaseOptions {
    /** display times in UTC */
    utc?: boolean;
}

/** Options for the `pulumi env version history` command. */
export interface PulumiEnvVersionHistoryOptions extends BaseOptions {
    /** the command to use to page through the environment's revisions */
    pager?: string;
    /** display times in UTC */
    utc?: boolean;
}

/** Options for the `pulumi env version retract` command. */
export interface PulumiEnvVersionRetractOptions extends BaseOptions {
    /** the reason for the retraction */
    reason?: string;
    /** the version to use to replace the retracted revision */
    replaceWith?: string;
}

/** Options for the `pulumi env version rollback` command. */
export interface PulumiEnvVersionRollbackOptions extends BaseOptions {
    /** set flag without a value (--draft) to create a draft rather than saving changes directly. --draft=<change-request-id> to update an existing change request. */
    draft?: string;
}

/** Options for the `pulumi env version tag` command. */
export interface PulumiEnvVersionTagOptions extends BaseOptions {
    /** display times in UTC */
    utc?: boolean;
}

/** Options for the `pulumi env version tag ls` command. */
export interface PulumiEnvVersionTagLsOptions extends BaseOptions {
    /** the command to use to page through the environment's version tags */
    pager?: string;
    /** display times in UTC */
    utc?: boolean;
}

/** Options for the `pulumi env version tag rm` command. */
export interface PulumiEnvVersionTagRmOptions extends BaseOptions {
}

/** Options for the `pulumi gen-completion` command. */
export interface PulumiGenCompletionOptions extends BaseOptions {
}

/** Options for the `pulumi gen-markdown` command. */
export interface PulumiGenMarkdownOptions extends BaseOptions {
}

/** Options for the `pulumi generate-cli-spec` command. */
export interface PulumiGenerateCliSpecOptions extends BaseOptions {
    /** help for generate-cli-spec */
    help?: boolean;
}

/** Options for the `pulumi help` command. */
export interface PulumiHelpOptions extends BaseOptions {
}

/** Options for the `pulumi import` command. */
export interface PulumiImportOptions extends BaseOptions {
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
    /** Do not calculate a preview before performing the import */
    skipPreview?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Suppress display of stack outputs (in case they contain sensitive values) */
    suppressOutputs?: boolean;
    /** Suppress display of the state permalink */
    suppressPermalink?: string;
    /** Suppress display of periodic progress dots */
    suppressProgress?: boolean;
    /** Automatically approve and perform the import after previewing it */
    yes?: boolean;
}

/** Options for the `pulumi install` command. */
export interface PulumiInstallOptions extends BaseOptions {
    /** Skip installing dependencies */
    noDependencies?: boolean;
    /** Skip installing plugins */
    noPlugins?: boolean;
    /** The max number of concurrent installs to perform. Parallelism of less then 1 implies unbounded parallelism */
    parallel?: number;
    /** Reinstall a plugin even if it already exists */
    reinstall?: boolean;
    /** Use language version tools to setup and install the language runtime */
    useLanguageVersionTools?: boolean;
}

/** Options for the `pulumi login` command. */
export interface PulumiLoginOptions extends BaseOptions {
    /** A cloud URL to log in to */
    cloudUrl?: string;
    /** A default org to associate with the login. Please note, currently, only the managed and self-hosted backends support organizations */
    defaultOrg?: string;
    /** Allow insecure server connections when using SSL */
    insecure?: boolean;
    /** Show interactive login options based on known accounts */
    interactive?: boolean;
    /** Use Pulumi in local-only mode */
    local?: boolean;
    /** The expiration for the cloud backend access token in duration format (e.g. '15m', '24h') */
    oidcExpiration?: string;
    /** The organization to use for OIDC token exchange audience */
    oidcOrg?: string;
    /** The team when exchanging for a team token */
    oidcTeam?: string;
    /** An OIDC token to exchange for a cloud backend access token. Can be either a raw token or a file path prefixed with 'file://'. */
    oidcToken?: string;
    /** The user when exchanging for a personal token */
    oidcUser?: string;
}

/** Options for the `pulumi logout` command. */
export interface PulumiLogoutOptions extends BaseOptions {
    /** Logout of all backends */
    all?: boolean;
    /** A cloud URL to log out of (defaults to current cloud) */
    cloudUrl?: string;
    /** Log out of using local mode */
    local?: boolean;
}

/** Options for the `pulumi logs` command. */
export interface PulumiLogsOptions extends BaseOptions {
    /** Use the configuration values in the specified file rather than detecting the file name */
    configFile?: string;
    /** Follow the log stream in real time (like tail -f) */
    follow?: boolean;
    /** Emit output as JSON */
    json?: boolean;
    /** Only return logs for the requested resource ('name', 'type::name' or full URN).  Defaults to returning all logs. */
    resource?: string;
    /** Only return logs newer than a relative duration ('5s', '2m', '3h') or absolute timestamp.  Defaults to returning the last 1 hour of logs. */
    since?: string;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi new` command. */
export interface PulumiNewOptions extends BaseOptions {
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
    /** Skip prompts and proceed with default values */
    yes?: boolean;
}

/** Options for the `pulumi org` command. */
export interface PulumiOrgOptions extends BaseOptions {
}

/** Options for the `pulumi org get-default` command. */
export interface PulumiOrgGetDefaultOptions extends BaseOptions {
}

/** Options for the `pulumi org search` command. */
export interface PulumiOrgSearchOptions extends BaseOptions {
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
}

/** Options for the `pulumi package` command. */
export interface PulumiPackageOptions extends BaseOptions {
}

/** Options for the `pulumi package add` command. */
export interface PulumiPackageAddOptions extends BaseOptions {
}

/** Options for the `pulumi package delete` command. */
export interface PulumiPackageDeleteOptions extends BaseOptions {
    /** Skip confirmation prompts, and proceed with deletion anyway */
    yes?: boolean;
}

/** Options for the `pulumi package gen-sdk` command. */
export interface PulumiPackageGenSdkOptions extends BaseOptions {
    /** The SDK language to generate: [nodejs|python|go|dotnet|java|all] */
    language?: string;
    /** Generate an SDK appropriate for local usage */
    local?: boolean;
    /** The directory to write the SDK to */
    out?: string;
    /** A folder of extra overlay files to copy to the generated SDK */
    overlays?: string;
    /** The provider plugin version to generate the SDK for */
    version?: string;
}

/** Options for the `pulumi package get-mapping` command. */
export interface PulumiPackageGetMappingOptions extends BaseOptions {
    /** The file to write the mapping data to */
    out?: string;
}

/** Options for the `pulumi package get-schema` command. */
export interface PulumiPackageGetSchemaOptions extends BaseOptions {
}

/** Options for the `pulumi package info` command. */
export interface PulumiPackageInfoOptions extends BaseOptions {
    /** Function name */
    function?: string;
    /** Module name */
    module?: string;
    /** Resource name */
    resource?: string;
}

/** Options for the `pulumi package pack-sdk` command. */
export interface PulumiPackagePackSdkOptions extends BaseOptions {
}

/** Options for the `pulumi package publish` command. */
export interface PulumiPackagePublishOptions extends BaseOptions {
    /** Path to the installation configuration markdown file */
    installationConfiguration?: string;
    /** The publisher of the package (e.g., 'pulumi'). Defaults to the publisher set in the package schema or the default organization in your pulumi config. */
    publisher?: string;
    /** Path to the package readme/index markdown file */
    readme?: string;
    /** The origin of the package (e.g., 'pulumi', 'private', 'opentofu'). Defaults to 'private'. */
    source?: string;
}

/** Options for the `pulumi package publish-sdk` command. */
export interface PulumiPackagePublishSdkOptions extends BaseOptions {
    /**
     * The path to the root of your package.
     * 	Example: ./sdk/nodejs
     * 	
     */
    path?: string;
}

/** Options for the `pulumi plugin` command. */
export interface PulumiPluginOptions extends BaseOptions {
}

/** Options for the `pulumi plugin install` command. */
export interface PulumiPluginInstallOptions extends BaseOptions {
    /** The expected SHA256 checksum for the plugin archive */
    checksum?: string;
    /** Force installation of an exact version match (usually >= is accepted) */
    exact?: boolean;
    /** Install a plugin from a binary, folder or tarball, instead of downloading it */
    file?: string;
    /** Reinstall a plugin even if it already exists */
    reinstall?: boolean;
    /** A URL to download plugins from */
    server?: string;
}

/** Options for the `pulumi plugin ls` command. */
export interface PulumiPluginLsOptions extends BaseOptions {
    /** Emit output as JSON */
    json?: boolean;
    /** List only the plugins used by the current project */
    project?: boolean;
}

/** Options for the `pulumi plugin rm` command. */
export interface PulumiPluginRmOptions extends BaseOptions {
    /** Remove all plugins */
    all?: boolean;
    /** Skip confirmation prompts, and proceed with removal anyway */
    yes?: boolean;
}

/** Options for the `pulumi plugin run` command. */
export interface PulumiPluginRunOptions extends BaseOptions {
    /** The plugin kind */
    kind?: string;
}

/** Options for the `pulumi policy` command. */
export interface PulumiPolicyOptions extends BaseOptions {
}

/** Options for the `pulumi policy disable` command. */
export interface PulumiPolicyDisableOptions extends BaseOptions {
    /** The Policy Group for which the Policy Pack will be disabled; if not specified, the default Policy Group is used */
    policyGroup?: string;
    /** The version of the Policy Pack that will be disabled; if not specified, any enabled version of the Policy Pack will be disabled */
    version?: string;
}

/** Options for the `pulumi policy enable` command. */
export interface PulumiPolicyEnableOptions extends BaseOptions {
    /** The file path for the Policy Pack configuration file */
    config?: string;
    /** The Policy Group for which the Policy Pack will be enabled; if not specified, the default Policy Group is used */
    policyGroup?: string;
}

/** Options for the `pulumi policy group` command. */
export interface PulumiPolicyGroupOptions extends BaseOptions {
}

/** Options for the `pulumi policy group ls` command. */
export interface PulumiPolicyGroupLsOptions extends BaseOptions {
    /** Emit output as JSON */
    json?: boolean;
}

/** Options for the `pulumi policy install` command. */
export interface PulumiPolicyInstallOptions extends BaseOptions {
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi policy ls` command. */
export interface PulumiPolicyLsOptions extends BaseOptions {
    /** Emit output as JSON */
    json?: boolean;
}

/** Options for the `pulumi policy new` command. */
export interface PulumiPolicyNewOptions extends BaseOptions {
    /** The location to place the generated Policy Pack; if not specified, the current directory is used */
    dir?: string;
    /** Forces content to be generated even if it would change existing files */
    force?: boolean;
    /** Generate the Policy Pack only; do not install dependencies */
    generateOnly?: boolean;
    /** Use locally cached templates without making any network requests */
    offline?: boolean;
}

/** Options for the `pulumi policy publish` command. */
export interface PulumiPolicyPublishOptions extends BaseOptions {
}

/** Options for the `pulumi policy rm` command. */
export interface PulumiPolicyRmOptions extends BaseOptions {
    /** Skip confirmation prompts, and proceed with removal anyway */
    yes?: boolean;
}

/** Options for the `pulumi policy validate-config` command. */
export interface PulumiPolicyValidateConfigOptions extends BaseOptions {
    /** The file path for the Policy Pack configuration file */
    config: string;
}

/** Options for the `pulumi preview` command. */
export interface PulumiPreviewOptions extends BaseOptions {
    /** Enable the ability to attach a debugger to the program and source based plugins being executed. Can limit debug type to 'program', 'plugins', 'plugin:<name>' or 'all'. */
    attachDebugger?: string[];
    /** The address of an existing language runtime host to connect to */
    client?: string;
    /** Config to use during the preview and save to the stack config file */
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
    /** Specify a resource URN to ignore. These resources will not be updated. Multiple resources can be specified using --exclude urn1 --exclude urn2. Wildcards (*, **) are also supported */
    exclude?: string[];
    /** Allow ignoring of dependent targets discovered but not specified in --exclude list */
    excludeDependents?: boolean;
    execAgent?: string;
    execKind?: string;
    /** Return an error if any changes are proposed by this preview */
    expectNoChanges?: boolean;
    /** Save any creates seen during the preview into an import file to use with 'pulumi import' */
    importFile?: string;
    /** Serialize the preview diffs, operations, and overall output as JSON. Set PULUMI_ENABLE_STREAMING_JSON_PREVIEW to stream JSON events instead. */
    json?: boolean;
    /** Optional message to associate with the preview operation */
    message?: string;
    /** Enable Pulumi Neo's assistance for improved CLI experience and insights (can also be set with PULUMI_NEO environment variable) */
    neo?: boolean;
    /** Start a Neo task to help debug errors that occur during the operation */
    neoTaskOnFailure?: boolean;
    /** Allow P resource operations to run in parallel at once (1 for no parallelism). */
    parallel?: number;
    /** Run one or more policy packs as part of this update */
    policyPack?: string[];
    /** Path to JSON file containing the config for the policy pack of the corresponding "--policy-pack" flag */
    policyPackConfig?: string[];
    /** Refresh the state of the stack's resources before this update */
    refresh?: string;
    /** Specify resources to replace. Multiple resources can be specified using --replace urn1 --replace urn2 */
    replace?: string[];
    /** Run the program to determine up-to-date state for providers to refresh resources, this only applies if --refresh is set */
    runProgram?: boolean;
    /** [PREVIEW] Save the operations proposed by the preview to a plan file at the given path */
    savePlan?: string;
    /** Show configuration keys and variables */
    showConfig?: boolean;
    /** Display full length of inputs & outputs */
    showFullOutput?: boolean;
    /** Show per-resource policy remediation details instead of a summary */
    showPolicyRemediations?: boolean;
    /** Show resources that are being read in, alongside those being managed directly in the stack */
    showReads?: boolean;
    /** Show detailed resource replacement creates and deletes instead of a single step */
    showReplacementSteps?: boolean;
    /** Show resources that needn't be updated because they haven't changed, alongside those that do */
    showSames?: boolean;
    /** Show secrets in plaintext in the CLI output, if used with --save-plan the secrets will also be shown in the plan file. Defaults to `false` */
    showSecrets?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Suppress display of stack outputs (in case they contain sensitive values) */
    suppressOutputs?: boolean;
    /** Suppress display of the state permalink */
    suppressPermalink?: string;
    /** Suppress display of periodic progress dots */
    suppressProgress?: boolean;
    /** Specify a single resource URN to update. Other resources will not be updated. Multiple resources can be specified using --target urn1 --target urn2 */
    target?: string[];
    /** Allow updating of dependent targets discovered but not specified in --target list */
    targetDependents?: boolean;
    /** Specify a single resource URN to replace. Other resources will not be updated. Shorthand for --target urn --replace urn. */
    targetReplace?: string[];
}

/** Options for the `pulumi project` command. */
export interface PulumiProjectOptions extends BaseOptions {
}

/** Options for the `pulumi project ls` command. */
export interface PulumiProjectLsOptions extends BaseOptions {
    /** Emit output as JSON */
    json?: boolean;
    /** The organization whose projects to list */
    organization?: string;
}

/** Options for the `pulumi refresh` command. */
export interface PulumiRefreshOptions extends BaseOptions {
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
    /** Skip importing pending creates in interactive mode */
    skipPendingCreates?: boolean;
    /** Do not calculate a preview before performing the refresh */
    skipPreview?: boolean;
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
    /** Automatically approve and perform the refresh after previewing it */
    yes?: boolean;
}

/** Options for the `pulumi replay-events` command. */
export interface PulumiReplayEventsOptions extends BaseOptions {
    /** Print detailed debugging output during resource operations */
    debug?: boolean;
    /** Delay display by the given duration. Useful for attaching a debugger. */
    delay?: string;
    /** Display operation as a rich diff showing the overall change */
    diff?: boolean;
    /** Serialize the preview diffs, operations, and overall output as JSON */
    json?: boolean;
    /** Delay each event by the given duration. */
    period?: string;
    /** Must be set for events from a `pulumi preview`. */
    preview?: boolean;
    /** Show configuration keys and variables */
    showConfig?: boolean;
    /** Show resources that are being read in, alongside those being managed directly in the stack */
    showReads?: boolean;
    /** Show detailed resource replacement creates and deletes instead of a single step */
    showReplacementSteps?: boolean;
    /** Show resources that needn't be updated because they haven't changed, alongside those that do */
    showSames?: boolean;
    /** Suppress display of stack outputs (in case they contain sensitive values) */
    suppressOutputs?: boolean;
    /** Suppress display of periodic progress dots */
    suppressProgress?: boolean;
}

/** Options for the `pulumi schema` command. */
export interface PulumiSchemaOptions extends BaseOptions {
}

/** Options for the `pulumi schema check` command. */
export interface PulumiSchemaCheckOptions extends BaseOptions {
    /** Whether references to nonexistent types should be considered errors */
    allowDanglingReferences?: boolean;
}

/** Options for the `pulumi stack` command. */
export interface PulumiStackOptions extends BaseOptions {
    /** Display each resource's provider-assigned unique ID */
    showIds?: boolean;
    /** Display only the stack name */
    showName?: boolean;
    /** Display stack outputs which are marked as secret in plaintext */
    showSecrets?: boolean;
    /** Display each resource's Pulumi-assigned globally unique URN */
    showUrns?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi stack change-secrets-provider` command. */
export interface PulumiStackChangeSecretsProviderOptions extends BaseOptions {
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi stack export` command. */
export interface PulumiStackExportOptions extends BaseOptions {
    /** A filename to write stack output to */
    file?: string;
    /** Emit secrets in plaintext in exported stack. Defaults to `false` */
    showSecrets?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Previous stack version to export. (If unset, will export the latest.) */
    version?: string;
}

/** Options for the `pulumi stack graph` command. */
export interface PulumiStackGraphOptions extends BaseOptions {
    /** Sets the color of dependency edges in the graph */
    dependencyEdgeColor?: string;
    /** An optional DOT fragment that will be inserted at the top of the digraph element. This can be used for styling the graph elements, setting graph properties etc. */
    dotFragment?: string;
    /** Ignores edges introduced by dependency resource relationships */
    ignoreDependencyEdges?: boolean;
    /** Ignores edges introduced by parent/child resource relationships */
    ignoreParentEdges?: boolean;
    /** Sets the color of parent edges in the graph */
    parentEdgeColor?: string;
    /** Sets the resource name as the node label for each node of the graph */
    shortNodeName?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi stack history` command. */
export interface PulumiStackHistoryOptions extends BaseOptions {
    /** Show full dates, instead of relative dates */
    fullDates?: boolean;
    /** Emit output as JSON */
    json?: boolean;
    /** Used with 'page-size' to paginate results */
    page?: number;
    /** Used with 'page' to control number of results returned */
    pageSize?: number;
    /** Show secret values when listing config instead of displaying blinded values */
    showSecrets?: boolean;
    /** Choose a stack other than the currently selected one */
    stack?: string;
}

/** Options for the `pulumi stack import` command. */
export interface PulumiStackImportOptions extends BaseOptions {
    /** A filename to read stack input from */
    file?: string;
    /** Force the import to occur, even if apparent errors are discovered beforehand (not recommended) */
    force?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi stack init` command. */
export interface PulumiStackInitOptions extends BaseOptions {
    /** The name of the stack to copy existing config from */
    copyConfigFrom?: string;
    /** Do not select the stack */
    noSelect?: boolean;
    /** Store stack configuration remotely */
    remoteConfig?: boolean;
    /**
     * The type of the provider that should be used to encrypt and decrypt secrets
     * (possible choices: default, passphrase, awskms, azurekeyvault, gcpkms, hashivault)
     */
    secretsProvider?: string;
    /** The name of the stack to create */
    stack?: string;
    /** A list of team names that should have permission to read and update this stack, once created */
    teams?: string[];
}

/** Options for the `pulumi stack ls` command. */
export interface PulumiStackLsOptions extends BaseOptions {
    /** List all stacks instead of just stacks for the current project */
    all?: boolean;
    /** Emit output as JSON */
    json?: boolean;
    /** Filter returned stacks to those in a specific organization */
    organization?: string;
    /** Filter returned stacks to those with a specific project name */
    project?: string;
    /** Filter returned stacks to those in a specific tag (tag-name or tag-name=tag-value) */
    tag?: string;
}

/** Options for the `pulumi stack output` command. */
export interface PulumiStackOutputOptions extends BaseOptions {
    /** Emit output as JSON */
    json?: boolean;
    /** Emit output as a shell script */
    shell?: boolean;
    /** Display outputs which are marked as secret in plaintext */
    showSecrets?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi stack rename` command. */
export interface PulumiStackRenameOptions extends BaseOptions {
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi stack rm` command. */
export interface PulumiStackRmOptions extends BaseOptions {
    /** Forces deletion of the stack, leaving behind any resources managed by the stack */
    force?: boolean;
    /** Do not delete the corresponding Pulumi.<stack-name>.yaml configuration file for the stack */
    preserveConfig?: boolean;
    /** Additionally remove backups of the stack, if using the DIY backend */
    removeBackups?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Skip confirmation prompts, and proceed with removal anyway */
    yes?: boolean;
}

/** Options for the `pulumi stack select` command. */
export interface PulumiStackSelectOptions extends BaseOptions {
    /** If selected stack does not exist, create it */
    create?: boolean;
    /**
     * Use with --create flag, The type of the provider that should be used to encrypt and decrypt secrets
     * (possible choices: default, passphrase, awskms, azurekeyvault, gcpkms, hashivault)
     */
    secretsProvider?: string;
    /** The name of the stack to select */
    stack?: string;
}

/** Options for the `pulumi stack tag` command. */
export interface PulumiStackTagOptions extends BaseOptions {
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi stack tag get` command. */
export interface PulumiStackTagGetOptions extends BaseOptions {
}

/** Options for the `pulumi stack tag ls` command. */
export interface PulumiStackTagLsOptions extends BaseOptions {
    /** Emit output as JSON */
    json?: boolean;
}

/** Options for the `pulumi stack tag rm` command. */
export interface PulumiStackTagRmOptions extends BaseOptions {
}

/** Options for the `pulumi stack tag set` command. */
export interface PulumiStackTagSetOptions extends BaseOptions {
}

/** Options for the `pulumi stack unselect` command. */
export interface PulumiStackUnselectOptions extends BaseOptions {
}

/** Options for the `pulumi state` command. */
export interface PulumiStateOptions extends BaseOptions {
}

/** Options for the `pulumi state delete` command. */
export interface PulumiStateDeleteOptions extends BaseOptions {
    /** Delete all resources in the stack */
    all?: boolean;
    /** Force deletion of protected resources */
    force?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Delete the URN and all its dependents */
    targetDependents?: boolean;
    /** Skip confirmation prompts */
    yes?: boolean;
}

/** Options for the `pulumi state edit` command. */
export interface PulumiStateEditOptions extends BaseOptions {
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi state move` command. */
export interface PulumiStateMoveOptions extends BaseOptions {
    /** The name of the stack to move resources to */
    dest?: string;
    /** Include all the parents of the moved resources as well */
    includeParents?: boolean;
    /** The name of the stack to move resources from */
    source?: string;
    /** Automatically approve and perform the move */
    yes?: boolean;
}

/** Options for the `pulumi state protect` command. */
export interface PulumiStateProtectOptions extends BaseOptions {
    /** Protect all resources in the checkpoint */
    all?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Skip confirmation prompts */
    yes?: boolean;
}

/** Options for the `pulumi state rename` command. */
export interface PulumiStateRenameOptions extends BaseOptions {
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Skip confirmation prompts */
    yes?: boolean;
}

/** Options for the `pulumi state repair` command. */
export interface PulumiStateRepairOptions extends BaseOptions {
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Automatically approve and perform the repair */
    yes?: boolean;
}

/** Options for the `pulumi state taint` command. */
export interface PulumiStateTaintOptions extends BaseOptions {
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Skip confirmation prompts */
    yes?: boolean;
}

/** Options for the `pulumi state unprotect` command. */
export interface PulumiStateUnprotectOptions extends BaseOptions {
    /** Unprotect all resources in the checkpoint */
    all?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Skip confirmation prompts */
    yes?: boolean;
}

/** Options for the `pulumi state untaint` command. */
export interface PulumiStateUntaintOptions extends BaseOptions {
    /** Untaint all resources in the checkpoint */
    all?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Skip confirmation prompts */
    yes?: boolean;
}

/** Options for the `pulumi state upgrade` command. */
export interface PulumiStateUpgradeOptions extends BaseOptions {
    /** Automatically approve and perform the upgrade */
    yes?: boolean;
}

/** Options for the `pulumi template` command. */
export interface PulumiTemplateOptions extends BaseOptions {
}

/** Options for the `pulumi template publish` command. */
export interface PulumiTemplatePublishOptions extends BaseOptions {
    /** The name of the template (required) */
    name: string;
    /** The publisher of the template (e.g., 'pulumi'). Defaults to the default organization in your pulumi config. */
    publisher?: string;
    /** The version of the template (required, semver format) */
    version: string;
}

/** Options for the `pulumi up` command. */
export interface PulumiUpOptions extends BaseOptions {
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
    /** Serialize the update diffs, operations, and overall output as JSON */
    json?: boolean;
    /** Optional message to associate with the update operation */
    message?: string;
    /** Enable Pulumi Neo's assistance for improved CLI experience and insights (can also be set with PULUMI_NEO environment variable) */
    neo?: boolean;
    /** Start a Neo task to help debug errors that occur during the operation */
    neoTaskOnFailure?: boolean;
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
    /** Do not calculate a preview before performing the update */
    skipPreview?: boolean;
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
    /** Automatically approve and perform the update after previewing it */
    yes?: boolean;
}

/** Options for the `pulumi version` command. */
export interface PulumiVersionOptions extends BaseOptions {
}

/** Options for the `pulumi view-trace` command. */
export interface PulumiViewTraceOptions extends BaseOptions {
    /** the port the trace viewer will listen on */
    port?: number;
}

/** Options for the `pulumi watch` command. */
export interface PulumiWatchOptions extends BaseOptions {
    /** Config to use during the update */
    config?: string[];
    /** Use the configuration values in the specified file rather than detecting the file name */
    configFile?: string;
    /** Config keys contain a path to a property in a map or list to set */
    configPath?: boolean;
    /** Print detailed debugging output during resource operations */
    debug?: boolean;
    execKind?: string;
    /** Optional message to associate with each update operation */
    message?: string;
    /** Allow P resource operations to run in parallel at once (1 for no parallelism). */
    parallel?: number;
    /** Specify one or more relative or absolute paths that need to be watched. A path can point to a folder or a file. Defaults to working directory */
    path?: string[];
    /** Run one or more policy packs as part of each update */
    policyPack?: string[];
    /** Path to JSON file containing the config for the policy pack of the corresponding "--policy-pack" flag */
    policyPackConfig?: string[];
    /** Refresh the state of the stack's resources before each update */
    refresh?: boolean;
    /** The type of the provider that should be used to encrypt and decrypt secrets (possible choices: default, passphrase, awskms, azurekeyvault, gcpkms, hashivault). Only used when creating a new stack from an existing template */
    secretsProvider?: string;
    /** Show configuration keys and variables */
    showConfig?: boolean;
    /** Show detailed resource replacement creates and deletes instead of a single step */
    showReplacementSteps?: boolean;
    /** Show resources that don't need be updated because they haven't changed, alongside those that do */
    showSames?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi whoami` command. */
export interface PulumiWhoamiOptions extends BaseOptions {
    /** Emit output as JSON */
    json?: boolean;
    /** Print detailed whoami information */
    verbose?: boolean;
}
