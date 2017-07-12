// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as slack from "@slack/client";
import {builders, providers} from "./cicd";
import * as github from "./github";

// On pushes or PR merges,
//     - In master, build and deploy the bits to production.
//     - In staging (and anywhere else), just build and test the bits.
// GitHub's package handles updating the commit status accordingly.
github.webhooks.onPush(async (e: github.webhooks.PushEvent) => {
    let sha: string = e.commit;
    let branch: string = e.ref;
    if (branch === "master") {
        let image: string = `us.gcr.io/${e.repository}:${sha}`;
        console.log(`{sha}: master branch: building and deploying ${image} to production`);
        await builders.docker.build({ tag: sha }); // build the image and tag it with the SHA.
        await builders.docker.push(image); // now push that newly tagged image to GCR.
        await providers.gcloud.rollingUpdate({ name: image }); // finally, do a rolling deploy in GKE.
    }
    else {
        console.log(`${sha}: non-master branch ${branch}: building and testing`);
        await builders.go.test(); // test Go bits without Dockerization.
        await builders.docker.build(); // build the full Docker container.
    }
});

// On PR request getting opened, run golint and flag any errors.
github.webhooks.onPullRequest(async (e: github.webhooks.PullRequestEvent) => {
    await builders.go.golint(); // run Golint and, if there are errors, log them.
});

// On creation of a new issue, post to our Slack channel.
github.webhooks.onIssueOpened(async (e: github.webhooks.IssueEvent) => {
    await slack.postMessage({
        channel: "Issues",
        message: `New issue ${e.issue.title} (#${e.issue.number}) `+
            `by ${e.issue.user}: ${e.issue.url}`,
    });
});

