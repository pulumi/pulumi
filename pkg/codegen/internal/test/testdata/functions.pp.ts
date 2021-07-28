import * as pulumi from "@pulumi/pulumi";

const encoded = Buffer.from("haha business").toString("base64");
const joined = [
    "haha",
    "business",
].join("-");
