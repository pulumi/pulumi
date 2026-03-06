import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

const noTimeouts = new simple.Resource("noTimeouts", {value: true});
const createOnly = new simple.Resource("createOnly", {value: true}, {
    customTimeouts: {
        create: "5m",
    },
});
const updateOnly = new simple.Resource("updateOnly", {value: true}, {
    customTimeouts: {
        update: "10m",
    },
});
const deleteOnly = new simple.Resource("deleteOnly", {value: true}, {
    customTimeouts: {
        "delete": "3m",
    },
});
const allTimeouts = new simple.Resource("allTimeouts", {value: true}, {
    customTimeouts: {
        create: "2m",
        update: "4m",
        "delete": "1m",
    },
});
