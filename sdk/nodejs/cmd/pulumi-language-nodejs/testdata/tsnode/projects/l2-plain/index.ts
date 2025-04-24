import * as pulumi from "@pulumi/pulumi";
import * as plain from "@pulumi/plain";

const res = new plain.Resource("res", {data: {
    innerData: {
        boolean: false,
        float: 2.17,
        integer: -12,
        string: "Goodbye",
        boolArray: [
            false,
            true,
        ],
        stringMap: {
            two: "turtle doves",
            three: "french hens",
        },
    },
    boolean: true,
    float: 4.5,
    integer: 1024,
    string: "Hello",
    boolArray: [
        true,
        false,
    ],
    stringMap: {
        x: "100",
        y: "200",
    },
}});
