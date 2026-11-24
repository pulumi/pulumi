import * as pulumi from "@pulumi/pulumi";
import { MyComponent } from "./myComponent";

const cmp = new MyComponent("cmp", {booleanMap: {
    "my key": false,
    "my.key": true,
    "my-key": false,
    my_key: true,
    MY_KEY: false,
    myKey: true,
    __type: true,
    __internal: false,
    __provider: true,
    __version: false,
    "": true,
    "Some ${common} \"characters\" 'that' need escaping: \\ (backslash), \x09 (tab), \x1b (escape), \x07 (bell), \x00 (null), \u{e0021} (tag space)": false,
    "Format and glob specifiers: %percent ...ellipsis {open }close *asterisk ?question ,comma &&and ||or !not =>arrow ==equal :colon /slash": true,
}});
export const resourceBooleanMap = cmp.booleanMap;
