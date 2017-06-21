// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import * as aws from "@lumi/aws";

export interface PushEvent {
    commit: string;
    ref: string;
    repository: string;
}

export interface PullRequestEvent {
}

export interface IssueEvent {
    issue: Issue;
}

export interface Issue {
    title: string;
    user: string;
    url: string;
    number: string;
}

export class WebHooks {

    onPush(f: (e: PushEvent) => void): void {
        console.log("Not yet implemented");
    }
    onPullRequest(f: (e: PullRequestEvent) => void): void {
        console.log("Not yet implemented");
    }

    onIssueOpened(f: (e: IssueEvent, callback: (err: any, res: any) => void) => void): void {
        // TODO: This is a mock of what the real GitHub provider will do.
        let func = new aws.serverless.Function(
            "f",
            [aws.iam.AWSLambdaFullAccess],
            (event, context, callback) => {
                f(
                    {
                        issue: {
                            number: "230",
                            title: "[lumi] Unify module and global scopes with the lexical scope chain",
                            url: "https://github.com/pulumi/lumi/issues/230",
                            user: "lukehoban",
                        },
                    },
                    callback,
                );
                console.log(context);
            },
        );
    }
}

export let webhooks = new WebHooks();
