import * as pulumi from "@pulumi/pulumi";
import * as unionref from "@pulumi/unionref";

const github = new unionref.Github("github", {});
const gitlab = new unionref.Gitlab("gitlab", {});
const pipeline = new unionref.Pipeline("pipeline", {
    source: github,
    program: {
        source: gitlab,
    },
    sources: [
        github,
        gitlab,
    ],
});
