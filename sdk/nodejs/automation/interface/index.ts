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

    deploymentCancel(options: PulumiDeploymentCancelOptions, deploymentId: string): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("deployment");
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

        if (options.output != null) {
            __flags.push("--output", "" + options.output);
        }

        if (options.stack != null) {
            __flags.push("--stack", "" + options.stack);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        __arguments.push("" + deploymentId);
        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    deploymentGet(options: PulumiDeploymentGetOptions, deploymentVersion: string): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("deployment");
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

        if (options.stack != null) {
            __flags.push("--stack", "" + options.stack);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        __arguments.push("" + deploymentVersion);
        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    deploymentList(options: PulumiDeploymentListOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("deployment");
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

        if (options.asc) {
            __flags.push("--asc");
        }

        if (options.count != null) {
            __flags.push("--count", "" + options.count);
        }

        if (options.output != null) {
            __flags.push("--output", "" + options.output);
        }

        if (options.sort != null) {
            __flags.push("--sort", "" + options.sort);
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

    deploymentLog(options: PulumiDeploymentLogOptions, deploymentVersion: string): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("deployment");
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

        if (options.all) {
            __flags.push("--all");
        }

        if (options.count != null) {
            __flags.push("--count", "" + options.count);
        }

        if (options.job != null) {
            __flags.push("--job", "" + options.job);
        }

        if (options.offset != null) {
            __flags.push("--offset", "" + options.offset);
        }

        if (options.output != null) {
            __flags.push("--output", "" + options.output);
        }

        if (options.stack != null) {
            __flags.push("--stack", "" + options.stack);
        }

        if (options.step != null) {
            __flags.push("--step", "" + options.step);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        __arguments.push("" + deploymentVersion);
        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    deploymentRun(options: PulumiDeploymentRunOptions, operation: string, url?: string): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("deployment");
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

        if (options.agentPoolId != null) {
            __flags.push("--agent-pool-id", "" + options.agentPoolId);
        }

        for (const __item of options.env ?? []) {
            if (__item != null) {
                __flags.push("--env", "" + __item);
            }
        }

        for (const __item of options.envSecret ?? []) {
            if (__item != null) {
                __flags.push("--env-secret", "" + __item);
            }
        }

        if (options.executorImage != null) {
            __flags.push("--executor-image", "" + options.executorImage);
        }

        if (options.executorImagePassword != null) {
            __flags.push("--executor-image-password", "" + options.executorImagePassword);
        }

        if (options.executorImageUsername != null) {
            __flags.push("--executor-image-username", "" + options.executorImageUsername);
        }

        if (options.gitAuthAccessToken != null) {
            __flags.push("--git-auth-access-token", "" + options.gitAuthAccessToken);
        }

        if (options.gitAuthPassword != null) {
            __flags.push("--git-auth-password", "" + options.gitAuthPassword);
        }

        if (options.gitAuthSshPrivateKey != null) {
            __flags.push("--git-auth-ssh-private-key", "" + options.gitAuthSshPrivateKey);
        }

        if (options.gitAuthSshPrivateKeyPath != null) {
            __flags.push("--git-auth-ssh-private-key-path", "" + options.gitAuthSshPrivateKeyPath);
        }

        if (options.gitAuthUsername != null) {
            __flags.push("--git-auth-username", "" + options.gitAuthUsername);
        }

        if (options.gitBranch != null) {
            __flags.push("--git-branch", "" + options.gitBranch);
        }

        if (options.gitCommit != null) {
            __flags.push("--git-commit", "" + options.gitCommit);
        }

        if (options.gitRepoDir != null) {
            __flags.push("--git-repo-dir", "" + options.gitRepoDir);
        }

        if (options.inheritSettings) {
            __flags.push("--inherit-settings");
        }

        for (const __item of options.preRunCommand ?? []) {
            if (__item != null) {
                __flags.push("--pre-run-command", "" + __item);
            }
        }

        if (options.skipInstallDependencies) {
            __flags.push("--skip-install-dependencies");
        }

        if (options.stack != null) {
            __flags.push("--stack", "" + options.stack);
        }

        if (options.suppressPermalink) {
            __flags.push("--suppress-permalink");
        }

        if (options.suppressStreamLogs) {
            __flags.push("--suppress-stream-logs");
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        __arguments.push("" + operation);
        if (url != null) {
            __arguments.push("" + url);
        }
        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    deploymentSettingsDestroy(options: PulumiDeploymentSettingsDestroyOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("deployment");
        __final.push("settings");
        __final.push("destroy");

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

    deploymentSettingsEdit(options: PulumiDeploymentSettingsEditOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("deployment");
        __final.push("settings");
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

        if (options.branch != null) {
            __flags.push("--branch", "" + options.branch);
        }

        if (options.cache) {
            __flags.push("--cache");
        }

        if (options.commit != null) {
            __flags.push("--commit", "" + options.commit);
        }

        if (options.deleteAfterDestroy) {
            __flags.push("--delete-after-destroy");
        }

        if (options.deployTags) {
            __flags.push("--deploy-tags");
        }

        for (const __item of options.env ?? []) {
            if (__item != null) {
                __flags.push("--env", "" + __item);
            }
        }

        if (options.executorImage != null) {
            __flags.push("--executor-image", "" + options.executorImage);
        }

        if (options.executorRootPath != null) {
            __flags.push("--executor-root-path", "" + options.executorRootPath);
        }

        if (options.folder != null) {
            __flags.push("--folder", "" + options.folder);
        }

        if (options.gitAuthAccessToken != null) {
            __flags.push("--git-auth-access-token", "" + options.gitAuthAccessToken);
        }

        if (options.gitAuthPassword != null) {
            __flags.push("--git-auth-password", "" + options.gitAuthPassword);
        }

        if (options.gitAuthSshPrivateKey != null) {
            __flags.push("--git-auth-ssh-private-key", "" + options.gitAuthSshPrivateKey);
        }

        if (options.gitAuthSshPrivateKeyPassword != null) {
            __flags.push("--git-auth-ssh-private-key-password", "" + options.gitAuthSshPrivateKeyPassword);
        }

        if (options.gitAuthSshPrivateKeyPath != null) {
            __flags.push("--git-auth-ssh-private-key-path", "" + options.gitAuthSshPrivateKeyPath);
        }

        if (options.gitAuthUsername != null) {
            __flags.push("--git-auth-username", "" + options.gitAuthUsername);
        }

        if (options.gitUrl != null) {
            __flags.push("--git-url", "" + options.gitUrl);
        }

        if (options.githubRepo != null) {
            __flags.push("--github-repo", "" + options.githubRepo);
        }

        if (options.installationId != null) {
            __flags.push("--installation-id", "" + options.installationId);
        }

        if (options.oidcAwsClear) {
            __flags.push("--oidc-aws-clear");
        }

        if (options.oidcAwsDuration != null) {
            __flags.push("--oidc-aws-duration", "" + options.oidcAwsDuration);
        }

        for (const __item of options.oidcAwsPolicyArn ?? []) {
            if (__item != null) {
                __flags.push("--oidc-aws-policy-arn", "" + __item);
            }
        }

        if (options.oidcAwsRoleArn != null) {
            __flags.push("--oidc-aws-role-arn", "" + options.oidcAwsRoleArn);
        }

        if (options.oidcAwsSessionName != null) {
            __flags.push("--oidc-aws-session-name", "" + options.oidcAwsSessionName);
        }

        if (options.oidcAzureClear) {
            __flags.push("--oidc-azure-clear");
        }

        if (options.oidcAzureClientId != null) {
            __flags.push("--oidc-azure-client-id", "" + options.oidcAzureClientId);
        }

        if (options.oidcAzureSubscriptionId != null) {
            __flags.push("--oidc-azure-subscription-id", "" + options.oidcAzureSubscriptionId);
        }

        if (options.oidcAzureTenantId != null) {
            __flags.push("--oidc-azure-tenant-id", "" + options.oidcAzureTenantId);
        }

        if (options.oidcGcpClear) {
            __flags.push("--oidc-gcp-clear");
        }

        if (options.oidcGcpProjectNumber != null) {
            __flags.push("--oidc-gcp-project-number", "" + options.oidcGcpProjectNumber);
        }

        if (options.oidcGcpProviderId != null) {
            __flags.push("--oidc-gcp-provider-id", "" + options.oidcGcpProviderId);
        }

        if (options.oidcGcpRegion != null) {
            __flags.push("--oidc-gcp-region", "" + options.oidcGcpRegion);
        }

        if (options.oidcGcpServiceAccount != null) {
            __flags.push("--oidc-gcp-service-account", "" + options.oidcGcpServiceAccount);
        }

        if (options.oidcGcpTokenLifetime != null) {
            __flags.push("--oidc-gcp-token-lifetime", "" + options.oidcGcpTokenLifetime);
        }

        if (options.oidcGcpWorkloadPoolId != null) {
            __flags.push("--oidc-gcp-workload-pool-id", "" + options.oidcGcpWorkloadPoolId);
        }

        if (options.output != null) {
            __flags.push("--output", "" + options.output);
        }

        for (const __item of options.pathFilter ?? []) {
            if (__item != null) {
                __flags.push("--path-filter", "" + __item);
            }
        }

        if (options.prTemplate) {
            __flags.push("--pr-template");
        }

        for (const __item of options.preRunCommand ?? []) {
            if (__item != null) {
                __flags.push("--pre-run-command", "" + __item);
            }
        }

        if (options.previewPrs) {
            __flags.push("--preview-prs");
        }

        if (options.pushToDeploy) {
            __flags.push("--push-to-deploy");
        }

        if (options.remediateIfDriftDetected) {
            __flags.push("--remediate-if-drift-detected");
        }

        if (options.removeAllEnv) {
            __flags.push("--remove-all-env");
        }

        for (const __item of options.removeEnv ?? []) {
            if (__item != null) {
                __flags.push("--remove-env", "" + __item);
            }
        }

        if (options.removeGitAuth) {
            __flags.push("--remove-git-auth");
        }

        if (options.removeOidcAws) {
            __flags.push("--remove-oidc-aws");
        }

        if (options.removeOidcAzure) {
            __flags.push("--remove-oidc-azure");
        }

        if (options.removeOidcGcp) {
            __flags.push("--remove-oidc-gcp");
        }

        if (options.repo != null) {
            __flags.push("--repo", "" + options.repo);
        }

        for (const __item of options.reviewStackLabel ?? []) {
            if (__item != null) {
                __flags.push("--review-stack-label", "" + __item);
            }
        }

        if (options.runnerPool != null) {
            __flags.push("--runner-pool", "" + options.runnerPool);
        }

        for (const __item of options.secretEnv ?? []) {
            if (__item != null) {
                __flags.push("--secret-env", "" + __item);
            }
        }

        if (options.shell != null) {
            __flags.push("--shell", "" + options.shell);
        }

        if (options.skipInstallDeps) {
            __flags.push("--skip-install-deps");
        }

        if (options.skipIntermediateDeployments) {
            __flags.push("--skip-intermediate-deployments");
        }

        if (options.stack != null) {
            __flags.push("--stack", "" + options.stack);
        }

        for (const __item of options.tagFilter ?? []) {
            if (__item != null) {
                __flags.push("--tag-filter", "" + __item);
            }
        }

        if (options.templateSourceUrl != null) {
            __flags.push("--template-source-url", "" + options.templateSourceUrl);
        }

        if (options.vcsProvider != null) {
            __flags.push("--vcs-provider", "" + options.vcsProvider);
        }

        __final.push(...__flags);

        const __arguments: string[] = [];

        if (__arguments.length > 0) {
            __final.push("--");
            __final.push(...__arguments);
        }

        return this.__run(options, __final);
    }

    deploymentSettingsGet(options: PulumiDeploymentSettingsGetOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("deployment");
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

        if (options.output != null) {
            __flags.push("--output", "" + options.output);
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

    deploymentSettings(options: PulumiDeploymentSettingsOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("deployment");
        __final.push("settings");

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

    deployment(options: PulumiDeploymentOptions): ReturnType<API["__run"]> {
        const __final: string[] = [];
        __final.push("deployment");

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

/** Options for the `pulumi deployment` command. */
export interface PulumiDeploymentOptions extends BaseOptions {
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

/** Options for the `pulumi deployment cancel` command. */
export interface PulumiDeploymentCancelOptions extends BaseOptions {
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
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi deployment get` command. */
export interface PulumiDeploymentGetOptions extends BaseOptions {
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
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi deployment list` command. */
export interface PulumiDeploymentListOptions extends BaseOptions {
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
    /** Fetch all results (mutually exclusive with --count) */
    all?: boolean;
    /** Sort in ascending order (default descending) */
    asc?: boolean;
    /** Number of results to display (fetches multiple pages if needed) */
    count?: number;
    /** Output format. Supported values are: default and json */
    output?: string;
    /** The field to sort results by */
    sort?: string;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
}

/** Options for the `pulumi deployment log` command. */
export interface PulumiDeploymentLogOptions extends BaseOptions {
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
    /** Fetch every available log line, following server-side pagination; mutually exclusive with --count */
    all?: boolean;
    /** The number of log lines to fetch, 1-499 in step mode (0 to leave unset) */
    count?: number;
    /** The job index to fetch step-level logs for (-1 to leave unset) */
    job?: number;
    /** The offset within the step's logs (0 to leave unset) */
    offset?: number;
    /** Output format. Supported values are: default and json */
    output?: string;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** The step index within the job; requires --job (-1 to leave unset) */
    step?: number;
}

/** Options for the `pulumi deployment run` command. */
export interface PulumiDeploymentRunOptions extends BaseOptions {
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

/** Options for the `pulumi deployment settings destroy` command. */
export interface PulumiDeploymentSettingsDestroyOptions extends BaseOptions {
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

/** Options for the `pulumi deployment settings edit` command. */
export interface PulumiDeploymentSettingsEditOptions extends BaseOptions {
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
    /** Source branch */
    branch?: string;
    /** Cache dependencies between deployments */
    cache?: boolean;
    /** Source commit hash */
    commit?: string;
    /** Delete the stack after a successful destroy */
    deleteAfterDestroy?: boolean;
    /** Run updates for pushed tags (cannot be enabled together with --push-to-deploy) */
    deployTags?: boolean;
    /** Set a plaintext environment variable (repeatable, KEY=VALUE) */
    env?: string[];
    /** Custom executor image; empty string clears it to the default image */
    executorImage?: string;
    /** Executor root path; empty string clears it to the default (/) */
    executorRootPath?: string;
    /** Path to the Pulumi.yaml folder within the source repo */
    folder?: string;
    /** Git source: personal access token (pass --remove-git-auth to remove stored credentials) */
    gitAuthAccessToken?: string;
    /** Git source: basic auth password */
    gitAuthPassword?: string;
    /** Git source: PEM-encoded SSH private key, key material and not a path */
    gitAuthSshPrivateKey?: string;
    /** Git source: password for the SSH private key */
    gitAuthSshPrivateKeyPassword?: string;
    /** Git source: path to a PEM-encoded SSH private key file (mutually exclusive with --git-auth-ssh-private-key) */
    gitAuthSshPrivateKeyPath?: string;
    /** Git source: basic auth username */
    gitAuthUsername?: string;
    /** Git source: full repository URL (mutually exclusive with --repo) */
    gitUrl?: string;
    /** GitHub source: organization/repository */
    githubRepo?: string;
    /** Version control integration ID; only needed to choose between several integrations for the same provider. List them with: pulumi api ListAllVCSIntegrations -F orgName=<org> */
    installationId?: string;
    oidcAwsClear?: boolean;
    /** AWS OIDC: assume-role session duration (e.g. 30m, 1h) */
    oidcAwsDuration?: string;
    /** AWS OIDC: replace the session policy ARN list (repeatable or comma-separated) */
    oidcAwsPolicyArn?: string[];
    /** AWS OIDC: IAM role ARN to assume */
    oidcAwsRoleArn?: string;
    /** AWS OIDC: assume-role session name */
    oidcAwsSessionName?: string;
    oidcAzureClear?: boolean;
    /** Azure OIDC: federated workload identity client ID */
    oidcAzureClientId?: string;
    /** Azure OIDC: federated workload identity subscription ID */
    oidcAzureSubscriptionId?: string;
    /** Azure OIDC: federated workload identity tenant ID */
    oidcAzureTenantId?: string;
    oidcGcpClear?: boolean;
    /** GCP OIDC: numerical project number (e.g. 987654321) */
    oidcGcpProjectNumber?: string;
    /** GCP OIDC: identity provider ID within the workload pool */
    oidcGcpProviderId?: string;
    /** GCP OIDC: region */
    oidcGcpRegion?: string;
    /** GCP OIDC: service account email */
    oidcGcpServiceAccount?: string;
    /** GCP OIDC: lifetime of the temporary credentials (e.g. 30m, 1h) */
    oidcGcpTokenLifetime?: string;
    /** GCP OIDC: workload identity pool ID */
    oidcGcpWorkloadPoolId?: string;
    /** Output format. Supported values are: default and json */
    output?: string;
    /** Replace the path filter list (repeatable; pass once per filter); empty string clears it */
    pathFilter?: string[];
    /** Use this stack as a template for PR review stacks */
    prTemplate?: boolean;
    /** Replace the pre-run command list (repeatable; pass once per command); empty string clears it */
    preRunCommand?: string[];
    /** Run previews for pull requests */
    previewPrs?: boolean;
    /** Run updates for pushed commits (cannot be enabled together with --deploy-tags) */
    pushToDeploy?: boolean;
    /** Remediate the stack when a drift detection run finds drift */
    remediateIfDriftDetected?: boolean;
    /** Remove every environment variable */
    removeAllEnv?: boolean;
    /** Delete an environment variable by key (repeatable or comma-separated) */
    removeEnv?: string[];
    /** Remove the stored git credentials, whichever authentication mode they use */
    removeGitAuth?: boolean;
    /** AWS OIDC: remove the entire configuration */
    removeOidcAws?: boolean;
    /** Azure OIDC: remove the entire configuration */
    removeOidcAzure?: boolean;
    /** GCP OIDC: remove the entire configuration */
    removeOidcGcp?: boolean;
    /** Version control source: repository reference, e.g. organization/repository (mutually exclusive with --git-url) */
    repo?: string;
    /** GitHub only: replace the labels that trigger a PR review stack (repeatable); empty string clears them */
    reviewStackLabel?: string[];
    /** Deployment runner pool ID; empty string clears it to the Pulumi-hosted pool */
    runnerPool?: string;
    /** Set an encrypted environment variable (repeatable, KEY=VALUE) */
    secretEnv?: string[];
    /** Shell to use for pre-run commands */
    shell?: string;
    /** Skip automatic dependency installation */
    skipInstallDeps?: boolean;
    /** Skip intermediate deployments */
    skipIntermediateDeployments?: boolean;
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
    /** Replace the tag filter list (repeatable; pass once per filter); empty string clears it */
    tagFilter?: string[];
    /** Template source URL, e.g. registry://templates/source/acme/vpc; empty string clears it */
    templateSourceUrl?: string;
    /** Version control provider: github, gitlab, azure_devops, bitbucket or custom */
    vcsProvider?: string;
}

/** Options for the `pulumi deployment settings get` command. */
export interface PulumiDeploymentSettingsGetOptions extends BaseOptions {
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
    /** The name of the stack to operate on. Defaults to the current stack */
    stack?: string;
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
