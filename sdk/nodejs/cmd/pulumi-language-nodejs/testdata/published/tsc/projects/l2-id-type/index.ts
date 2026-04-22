import * as pulumi from "@pulumi/pulumi";
import * as primitive from "@pulumi/primitive";

// Test that the ID type is treated the same as a string type, despite being tracked as a distinct type. This 
// includes directly passing it to string fields, but also for bool and numeric values being able to cast to it.
const source1 = new primitive.Resource("source1", {
    boolean: false,
    float: 1,
    integer: 2,
    string: "1234",
    numberArray: [3],
    booleanMap: {
        source: false,
    },
});
const source2 = new primitive.Resource("source2", {
    boolean: false,
    float: 1,
    integer: 2,
    string: "true",
    numberArray: [3],
    booleanMap: {
        source: false,
    },
});
const idMap = {
    source1Token: source1.id,
    source2Token: source2.id,
};
const sink1 = new primitive.Resource("sink1", {
    boolean: false,
    float: idMap.source1Token.apply(x =>Number(x)),
    integer: idMap.source1Token.apply(x =>Number(x)),
    string: idMap.source1Token,
    numberArray: [idMap.source1Token.apply(x =>Number(x))],
    booleanMap: {
        sink: false,
    },
});
const sink2 = new primitive.Resource("sink2", {
    boolean: idMap.source2Token.apply(x =>x === "true"),
    float: 1,
    integer: 2,
    string: "abc",
    numberArray: [3],
    booleanMap: {
        sink: idMap.source2Token.apply(x =>x === "true"),
    },
});
export const ids = idMap;
