import * as pulumi from "@pulumi/pulumi";

export const empty = {};
export const strings = {
    greeting: "Hello, world!",
    farewell: "Goodbye, world!",
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
