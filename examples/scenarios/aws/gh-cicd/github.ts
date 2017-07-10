// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

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
            { policies: [aws.iam.AWSLambdaFullAccess] },
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
