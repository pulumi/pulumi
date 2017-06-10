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

//import {builders, providers} from "./cicd";
import * as github from "./github";  
import * as lumi from "@lumi/lumi";
// import * as slack from "@slack/client";

// // On pushes or PR merges,
// //     - In master, build and deploy the bits to production.
// //     - In staging (and anywhere else), just build and test the bits.
// // GitHub's package handles updating the commit status accordingly.
// github.webhooks.onPush(async (e: github.PushEvent) => {
//     let sha: string = e.commit;
//     let branch: string = e.ref;
//     if (branch === "master") {
//         let image: string = `us.gcr.io/${e.repository}:${sha}`;
//         console.log(`{sha}: master branch: building and deploying ${image} to production`);
//         await builders.docker.build({ tag: sha }); // build the image and tag it with the SHA.
//         await builders.docker.push(image); // now push that newly tagged image to GCR.
//         await providers.gcloud.rollingUpdate({ name: image }); // finally, do a rolling deploy in GKE.
//     }
//     else {
//         console.log(`${sha}: non-master branch ${branch}: building and testing`);
//         await builders.go.test(); // test Go bits without Dockerization.
//         await builders.docker.build(); // build the full Docker container.
//     }
// });

// // On PR request getting opened, run golint and flag any errors.
// github.webhooks.onPullRequest(async (e: github.PullRequestEvent) => {
//     await builders.go.golint(); // run Golint and, if there are errors, log them.
// });

let slackToken = "xoxp-159397724290-186784357158-195642376628-8a2c0a4dc8ca0e778df694a8fa010af8"
declare let require: any;

lumi.runtime.printf(`Hello ${slackToken}, good night.`);

// On creation of a new issue, post to our Slack channel.
github.webhooks.onIssueOpened((e: github.IssueEvent) => {
    let slack = require('@slack/client')
    let client = new slack.WebClient(slackToken)
    let message = `New issue ${e.issue.title} (#${e.issue.number}) by ${e.issue.user}: ${e.issue.url}`
    client.chat.postMessage("Issues", message, (err: any, result: any) => {
        if(!err) throw err;
        console.log(result);
    })
});