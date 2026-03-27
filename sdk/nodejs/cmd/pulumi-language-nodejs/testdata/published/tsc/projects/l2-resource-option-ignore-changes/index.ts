import * as pulumi from "@pulumi/pulumi";
import * as nestedobject from "@pulumi/nestedobject";

const receiverIgnore = new nestedobject.Receiver("receiverIgnore", {details: [{
    key: "a",
    value: "b",
}]}, {
    ignoreChanges: ["details[0].key"],
});
const mapIgnore = new nestedobject.MapContainer("mapIgnore", {tags: {
    env: "prod",
}}, {
    ignoreChanges: [
        "tags[\"env\"]",
        "tags[\"with.dot\"]",
        "tags[\"with escaped \\\"\"]",
    ],
});
const noIgnore = new nestedobject.Target("noIgnore", {name: "nothing"});
