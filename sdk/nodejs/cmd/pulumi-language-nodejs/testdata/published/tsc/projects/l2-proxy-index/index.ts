import * as pulumi from "@pulumi/pulumi";
import * as ref_ref from "@pulumi/ref-ref";

// Check we can index into properties of objects returned in outputs, this is similar to ref-ref but 
// we index into the outputs
const res = new ref_ref.Resource("res", {data: {
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
    boolArray: [true],
    stringMap: {
        x: "100",
        y: "200",
    },
}});
export const bool = res.data.boolean;
export const array = res.data.boolArray[0];
export const map = res.data.stringMap.x;
export const nested = res.data.innerData.stringMap.three;
