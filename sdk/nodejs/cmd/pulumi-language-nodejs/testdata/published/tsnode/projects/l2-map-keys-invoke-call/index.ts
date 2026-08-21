import * as pulumi from "@pulumi/pulumi";
import * as component from "@pulumi/component";
import * as simple_invoke from "@pulumi/simple-invoke";

const callable = new component.ComponentCallable("callable", {value: "unused"});
export const invokeResult = simple_invoke.echoMapOutput({
    stringMap: {
        "my key": "one",
        "my.key": "two",
        "my-key": "three",
        my_key: "four",
        MY_KEY: "five",
        myKey: "six",
        __type: "seven",
        __internal: "eight",
    },
}).stringMap;
export const callResult = callable.echoMap(({
    stringMap: {
        "my key": "one",
        "my.key": "two",
        "my-key": "three",
        my_key: "four",
        MY_KEY: "five",
        myKey: "six",
        __type: "seven",
        __internal: "eight",
    },
})).stringMap;
