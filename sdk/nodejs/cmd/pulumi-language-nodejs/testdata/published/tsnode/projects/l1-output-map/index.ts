import * as pulumi from "@pulumi/pulumi";

export const empty = {};
export const strings = {
    greeting: "Hello, world!",
    farewell: "Goodbye, world!",
};
export const adversarialStrings = {
    __type: "dunder type",
    __internal: "dunder internal",
    __provider: "dunder provider",
    __version: "dunder version",
    "": "empty key",
    "empty value": "",
    "dunder value": "__dunder",
    "Some ${common} \"characters\" 'that' need escaping: \\ (backslash), \x09 (tab), \x1b (escape), \x07 (bell), \x00 (null), \u{e0021} (tag space)": "Some ${common} \"characters\" 'that' need escaping: \\ (backslash), \x09 (tab), \x1b (escape), \x07 (bell), \x00 (null), \u{e0021} (tag space)",
};
export const numbers = {
    "1": 1,
    "2": 2,
};
export const keys = {
    "my.key": 1,
    "my-key": 2,
    my_key: 3,
    MY_KEY: 4,
    mykey: 5,
    MYKEY: 6,
};
