import * as pulumi from "@pulumi/pulumi";
import * as extended1 from "@pulumi/extended1";
import * as extended2 from "@pulumi/extended2";
import * as extension from "@pulumi/extension";

const baseCustomDefault = new extension.Custom("baseCustomDefault", {value: "baseCustomDefault"});
const baseExplicit = new extension.Provider("baseExplicit", {value: "baseExplicit"});
const baseCustomExplicit = new extension.Custom("baseCustomExplicit", {value: "baseCustomExplicit"}, {
    provider: baseExplicit,
});
// package "replaced" {
//   baseProviderName = "extension"
//   baseProviderVersion = "17.17.17"
//   replacement {
//     name = "replaced"
//     version = "34.34.34"
//     value = "cmVwbGFjZWQtcGFyYW1ldGVyCg==" // base64(utf8_bytes("replaced-parameter"))
//   }
// }
//
// resource "replacedCustomDefault" "replaced:index:Custom" {
//   value = "replacedCustomDefault"
// }
//
// resource "replacedExplicit" "pulumi:providers:replaced" {
//   value = "replacedExplicit"
// }
//
// resource "replacedCustomExplicit" "replaced:index:Custom" {
//   value = "replacedCustomExplicit"
//   options {
//     provider = replacedExplicit
//   }
// }
const extended1CustomDefault = new extended1.Custom("extended1CustomDefault", {value: "extended1CustomDefault"});
const extended1CustomExplicit = new extended1.Custom("extended1CustomExplicit", {value: "extended1CustomExplicit"}, {
    provider: baseExplicit,
});
const extended2CustomDefault = new extended2.Custom("extended2CustomDefault", {value: "extended2CustomDefault"});
const extended2CustomExplicit = new extended2.Custom("extended2CustomExplicit", {value: "extended2CustomExplicit"}, {
    provider: baseExplicit,
});
