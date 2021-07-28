import * as pulumi from "@pulumi/pulumi";

const encoded = (new Buffer("haha business")).toString("base64");
const joined = [
    "haha",
    "business",
].join("-");
