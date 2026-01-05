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

import assert from "assert";

import {
    fullyQualifiedStackName,
    isFullyQualifiedStackName,
    LocalWorkspace,
    LocalWorkspaceOptions,
    RemoteGitAuthArgs,
    RemoteGitProgramArgs,
    RemoteStack,
    RemoteWorkspace,
    RemoteWorkspaceOptions,
} from "../../automation";
import { getTestOrg, getTestSuffix } from "./util";

const testRepo = "https://github.com/pulumi/test-repo.git";

describe("RemoteWorkspace", () => {
    describe("remote cmd args", () => {
        const tests: {
            name: string;
            opts: LocalWorkspaceOptions;
            expected: string[];
        }[] = [
            {
                name: "empty",
                opts: {},
                expected: [],
            },
            {
                name: "just remote",
                opts: {
                    remote: true,
                },
                expected: ["--remote"],
            },
            {
                name: "url",
                opts: {
                    remote: true,
                    remoteGitProgramArgs: {
                        stackName: "stack",
                        url: "foo",
                    },
                },
                expected: ["--remote", "foo"],
            },
            {
                name: "path",
                opts: {
                    remote: true,
                    remoteGitProgramArgs: {
                        stackName: "stack",
                        url: "foo",
                        projectPath: "mypath",
                    },
                },
                expected: ["--remote", "foo", "--remote-git-repo-dir", "mypath"],
            },
            {
                name: "branch",
                opts: {
                    remote: true,
                    remoteGitProgramArgs: {
                        stackName: "stack",
                        url: "foo",
                        branch: "mybranch",
                    },
                },
                expected: ["--remote", "foo", "--remote-git-branch", "mybranch"],
            },
            {
                name: "commit",
                opts: {
                    remote: true,
                    remoteGitProgramArgs: {
                        stackName: "stack",
                        url: "foo",
                        commitHash: "mycommit",
                    },
                },
                expected: ["--remote", "foo", "--remote-git-commit", "mycommit"],
            },
            {
                name: "auth access token",
                opts: {
                    remote: true,
                    remoteGitProgramArgs: {
                        stackName: "stack",
                        url: "foo",
                        auth: {
                            personalAccessToken: "mytoken",
                        },
                    },
                },
                expected: ["--remote", "foo", "--remote-git-auth-access-token", "mytoken"],
            },
            {
                name: "auth ssh key",
                opts: {
                    remote: true,
                    remoteGitProgramArgs: {
                        stackName: "stack",
                        url: "foo",
                        auth: {
                            sshPrivateKey: "mykey",
                        },
                    },
                },
                expected: ["--remote", "foo", "--remote-git-auth-ssh-private-key", "mykey"],
            },
            {
                name: "auth ssh key path",
                opts: {
                    remote: true,
                    remoteGitProgramArgs: {
                        stackName: "stack",
                        url: "foo",
                        auth: {
                            sshPrivateKeyPath: "mykeypath",
                        },
                    },
                },
                expected: ["--remote", "foo", "--remote-git-auth-ssh-private-key-path", "mykeypath"],
            },
            {
                name: "auth ssh password",
                opts: {
                    remote: true,
                    remoteGitProgramArgs: {
                        stackName: "stack",
                        url: "foo",
                        auth: {
                            username: "myuser",
                            password: "mypass",
                        },
                    },
                },
                expected: [
                    "--remote",
                    "foo",
                    "--remote-git-auth-password",
                    "mypass",
                    "--remote-git-auth-username",
                    "myuser",
                ],
            },
            {
                name: "env",
                opts: {
                    remote: true,
                    remoteGitProgramArgs: {
                        stackName: "stack",
                        url: "foo",
                    },
                    remoteEnvVars: {
                        foo: "bar",
                    },
                },
                expected: ["--remote", "foo", "--remote-env", "foo=bar"],
            },
            {
                name: "env secret",
                opts: {
                    remote: true,
                    remoteGitProgramArgs: {
                        stackName: "stack",
                        url: "foo",
                    },
                    remoteEnvVars: {
                        foo: { secret: "bar" },
                    },
                },
                expected: ["--remote", "foo", "--remote-env-secret", "foo=bar"],
            },
            {
                name: "pre-run command",
                opts: {
                    remote: true,
                    remoteGitProgramArgs: {
                        stackName: "stack",
                        url: "foo",
                    },
                    remotePreRunCommands: ["whoami"],
                },
                expected: ["--remote", "foo", "--remote-pre-run-command", "whoami"],
            },
            {
                name: "skip install dependencies",
                opts: {
                    remote: true,
                    remoteGitProgramArgs: {
                        stackName: "stack",
                        url: "foo",
                    },
                    remoteSkipInstallDependencies: true,
                },
                expected: ["--remote", "foo", "--remote-skip-install-dependencies"],
            },
            {
                name: "inherit settings",
                opts: {
                    remote: true,
                    remoteGitProgramArgs: {
                        stackName: "stack",
                    },
                    remoteInheritSettings: true,
                },
                expected: ["--remote", "--remote-inherit-settings"],
            },
            {
                name: "remote image",
                opts: {
                    remote: true,
                    remoteExecutorImage: {
                        image: "test-image",
                    },
                },
                expected: ["--remote", "--remote-executor-image=test-image"],
            },
            {
                name: "remote image credentials",
                opts: {
                    remote: true,
                    remoteExecutorImage: {
                        image: "test-image",
                        credentials: {
                            username: "foo",
                            password: "bar",
                        },
                    },
                },
                expected: [
                    "--remote",
                    "--remote-executor-image=test-image",
                    "--remote-executor-image-username=foo",
                    "--remote-executor-image-password=bar",
                ],
            },
        ];
        tests.forEach((test) => {
            it(`${test.name}`, async () => {
                const ws = await LocalWorkspace.create(test.opts);
                const actual = ws.remoteArgs();
                assert.deepStrictEqual(actual, test.expected);
            });
        });
    });

    describe("selectStack", () => {
        describe("throws appropriate errors", () => testErrors(RemoteWorkspace.selectStack));
    });

    describe("createStack", () => {
        describe("throws appropriate errors", () => testErrors(RemoteWorkspace.createStack));

        it(`runs through the stack lifecycle`, async function () {
            // This test requires the service with access to Pulumi Deployments.
            // Set PULUMI_ACCESS_TOKEN to an access token with access to Pulumi Deployments
            // and set PULUMI_TEST_DEPLOYMENTS_API to any value to enable the test.
            if (!process.env.PULUMI_ACCESS_TOKEN) {
                this.skip();
                return;
            }
            if (!process.env.PULUMI_TEST_DEPLOYMENTS_API) {
                this.skip();
                return;
            }

            await testLifecycle(RemoteWorkspace.createStack);
        });
    });

    describe("createOrSelectStack", () => {
        describe("throws appropriate errors", () => testErrors(RemoteWorkspace.createOrSelectStack));

        it(`runs through the stack lifecycle`, async function () {
            // This test requires the service with access to Pulumi Deployments.
            // Set PULUMI_ACCESS_TOKEN to an access token with access to Pulumi Deployments
            // and set PULUMI_TEST_DEPLOYMENTS_API to any value to enable the test.
            if (!process.env.PULUMI_ACCESS_TOKEN) {
                this.skip();
                return;
            }
            if (!process.env.PULUMI_TEST_DEPLOYMENTS_API) {
                this.skip();
                return;
            }

            await testLifecycle(RemoteWorkspace.createOrSelectStack);
        });
    });
});

function testErrors(fn: (args: RemoteGitProgramArgs, opts?: RemoteWorkspaceOptions) => Promise<RemoteStack>) {
    const stack = "owner/project/stack";
    const tests: {
        name: string;
        stackName: string;
        url: string;
        branch?: string;
        commitHash?: string;
        auth?: RemoteGitAuthArgs;
        error: string;
    }[] = [
        {
            name: "stack empty",
            stackName: "",
            url: "",
            error: `stack name "" must be fully qualified.`,
        },
        {
            name: "stack just name",
            stackName: "name",
            url: "",
            error: `stack name "name" must be fully qualified.`,
        },
        {
            name: "stack just name & owner",
            stackName: "owner/name",
            url: "",
            error: `stack name "owner/name" must be fully qualified.`,
        },
        {
            name: "stack just sep",
            stackName: "/",
            url: "",
            error: `stack name "/" must be fully qualified.`,
        },
        {
            name: "stack just two seps",
            stackName: "//",
            url: "",
            error: `stack name "//" must be fully qualified.`,
        },
        {
            name: "stack just three seps",
            stackName: "///",
            url: "",
            error: `stack name "///" must be fully qualified.`,
        },
        {
            name: "stack invalid",
            stackName: "owner/project/stack/wat",
            url: "",
            error: `stack name "owner/project/stack/wat" must be fully qualified.`,
        },
        {
            name: "no url",
            stackName: stack,
            url: "",
            error: `url is required if inheritSettings is not set.`,
        },
        {
            name: "no branch or commit",
            stackName: stack,
            url: testRepo,
            error: `either branch or commitHash is required if inheritSettings is not set.`,
        },
        {
            name: "both branch and commit",
            stackName: stack,
            url: testRepo,
            branch: "branch",
            commitHash: "commit",
            error: `branch and commitHash cannot both be specified.`,
        },
        {
            name: "both ssh private key and path",
            stackName: stack,
            url: testRepo,
            branch: "branch",
            auth: {
                sshPrivateKey: "key",
                sshPrivateKeyPath: "path",
            },
            error: `sshPrivateKey and sshPrivateKeyPath cannot both be specified.`,
        },
    ];

    tests.forEach((test) => {
        it(`${test.name}`, async () => {
            const { stackName, url, branch, commitHash, auth } = test;
            await assert.rejects(
                async () => {
                    await fn({ stackName, url, branch, commitHash, auth });
                },
                {
                    message: test.error,
                },
            );
        });
    });
}

async function testLifecycle(fn: (args: RemoteGitProgramArgs, opts?: RemoteWorkspaceOptions) => Promise<RemoteStack>) {
    const stackName = fullyQualifiedStackName(getTestOrg(), "go_remote_proj", `int_test${getTestSuffix()}`);
    const stack = await fn(
        {
            stackName,
            url: testRepo,
            branch: "refs/heads/master",
            projectPath: "goproj",
        },
        {
            preRunCommands: [
                `pulumi config set bar abc --stack ${stackName}`,
                `pulumi config set --secret buzz secret --stack ${stackName}`,
            ],
            skipInstallDependencies: true,
        },
    );

    // pulumi up
    const upRes = await stack.up();
    assert.strictEqual(Object.keys(upRes.outputs).length, 3);
    assert.strictEqual(upRes.outputs["exp_static"].value, "foo");
    assert.strictEqual(upRes.outputs["exp_static"].secret, false);
    assert.strictEqual(upRes.outputs["exp_cfg"].value, "abc");
    assert.strictEqual(upRes.outputs["exp_cfg"].secret, false);
    assert.strictEqual(upRes.outputs["exp_secret"].value, "secret");
    assert.strictEqual(upRes.outputs["exp_secret"].secret, true);
    assert.strictEqual(upRes.summary.kind, "update");
    assert.strictEqual(upRes.summary.result, "succeeded");

    // pulumi preview
    const preRes = await stack.preview();
    assert.strictEqual(preRes.changeSummary.same, 1);

    // pulumi refresh
    const refRes = await stack.refresh();
    assert.strictEqual(refRes.summary.kind, "refresh");
    assert.strictEqual(refRes.summary.result, "succeeded");

    // pulumi destroy
    const destroyRes = await stack.destroy();
    assert.strictEqual(destroyRes.summary.kind, "destroy");
    assert.strictEqual(destroyRes.summary.result, "succeeded");

    await (await LocalWorkspace.create({})).removeStack(stackName);
}

describe("isFullyQualifiedStackName", () => {
    const tests = [
        {
            name: "fully qualified",
            input: "owner/project/stack",
            expected: true,
        },
        {
            name: "undefined",
            input: undefined,
            expected: false,
        },
        {
            name: "null",
            input: null,
            expected: false,
        },
        {
            name: "empty",
            input: "",
            expected: false,
        },
        {
            name: "name",
            input: "name",
            expected: false,
        },
        {
            name: "name & owner",
            input: "owner/name",
            expected: false,
        },
        {
            name: "sep",
            input: "/",
            expected: false,
        },
        {
            name: "two seps",
            input: "//",
            expected: false,
        },
        {
            name: "three seps",
            input: "///",
            expected: false,
        },
        {
            name: "invalid",
            input: "owner/project/stack/wat",
            expected: false,
        },
    ];

    tests.forEach((test) => {
        it(`${test.name}`, () => {
            const actual = isFullyQualifiedStackName(test.input!);
            assert.strictEqual(actual, test.expected);
        });
    });
});
