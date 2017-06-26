// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

/*tslint:disable:no-require-imports*/

import * as github from "./github";

export let slackToken = "<must provide a token>";
declare let require: any;

// On creation of a new issue, post to our Slack channel.
github.webhooks.onIssueOpened((e, callback) => {
    let slack = require("@slack/client");
    let client = new slack.WebClient(slackToken);
    let message = "New issue " + e.issue.title + " (#" + e.issue.number +") by "+ e.issue.user + ": " + e.issue.url;
    client.chat.postMessage("#issues", message, callback);
});
