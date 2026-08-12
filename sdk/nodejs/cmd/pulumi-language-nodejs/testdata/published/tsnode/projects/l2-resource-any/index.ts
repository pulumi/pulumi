import * as pulumi from "@pulumi/pulumi";
import * as any_handled from "@pulumi/any-handled";

const aString = new any_handled.Resource("aString", {value: "a string"});
const aBoolean = new any_handled.Resource("aBoolean", {value: true});
const aNumber = new any_handled.Resource("aNumber", {value: 42});
const aList = new any_handled.Resource("aList", {value: [
    1,
    true,
    "three",
]});
const anObject = new any_handled.Resource("anObject", {value: {
    key: "value",
    nested: {
        count: 1,
    },
}});
const anAsset = new any_handled.Resource("anAsset", {value: new pulumi.asset.StringAsset("the asset contents")});
