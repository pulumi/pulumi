import * as pulumi from "@pulumi/pulumi";
import * as primitive from "@pulumi/primitive";

const res = new primitive.Resource("res", {
    b: true,
    f: 3.14,
    i: 42,
    s: "hello",
    a: [
        -1,
        0,
        1,
    ],
    m: {
        t: true,
        f: false,
    },
});
