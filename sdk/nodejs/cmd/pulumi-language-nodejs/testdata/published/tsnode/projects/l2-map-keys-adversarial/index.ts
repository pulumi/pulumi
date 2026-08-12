import * as pulumi from "@pulumi/pulumi";
import * as primitive from "@pulumi/primitive";

const res = new primitive.Resource("res", {
    boolean: false,
    float: 2.17,
    integer: -12,
    string: "adversarial",
    numberArray: [
        0,
        1,
    ],
    booleanMap: {
        __type: true,
        __internal: false,
        __provider: true,
        __version: false,
        "": true,
        "Some ${common} \"characters\" 'that' need escaping: \\ (backslash), \x09 (tab), \x1b (escape), \x07 (bell), \x00 (null), \u{e0021} (tag space)": false,
        "Format and glob specifiers: %percent ...ellipsis {open }close *asterisk ?question ,comma &&and ||or !not =>arrow ==equal :colon /slash": true,
    },
});
const invokeResult = primitive.invokeOutput({
    boolean: false,
    float: 2.17,
    integer: -12,
    string: "adversarial",
    numberArray: [
        0,
        1,
    ],
    booleanMap: {
        __type: true,
        __internal: false,
        __provider: true,
        __version: false,
        "": true,
        "Some ${common} \"characters\" 'that' need escaping: \\ (backslash), \x09 (tab), \x1b (escape), \x07 (bell), \x00 (null), \u{e0021} (tag space)": false,
        "Format and glob specifiers: %percent ...ellipsis {open }close *asterisk ?question ,comma &&and ||or !not =>arrow ==equal :colon /slash": true,
    },
});
export const resourceBooleanMap = res.booleanMap;
export const invokeBooleanMap = invokeResult.booleanMap;
