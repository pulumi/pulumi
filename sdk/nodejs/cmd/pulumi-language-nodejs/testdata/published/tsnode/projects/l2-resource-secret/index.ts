import * as pulumi from "@pulumi/pulumi";
import * as secret from "@pulumi/secret";

const res = new secret.Resource("res", {
    "private": "closed",
    "public": "open",
    privateData: {
        "private": "closed",
        "public": "open",
    },
    publicData: {
        "private": "closed",
        "public": "open",
    },
    privateArray: ["closed"],
    privateMap: {
        key: "closed",
    },
    privateDataArray: [{
        "private": "closed",
        "public": "open",
    }],
    privateDataMap: {
        key: {
            "private": "closed",
            "public": "open",
        },
    },
});
