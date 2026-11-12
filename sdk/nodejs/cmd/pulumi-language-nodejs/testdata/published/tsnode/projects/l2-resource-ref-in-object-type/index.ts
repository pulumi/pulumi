import * as pulumi from "@pulumi/pulumi";
import * as objectref from "@pulumi/objectref";

const widget = new objectref.referencingfirst.Widget("widget", {settings: {
    name: "widget",
}});
const gadget = new objectref.referencingfirst.Gadget("gadget", {widgetRef: {
    widget: widget,
}});
const alpha = new objectref.referencedfirst.Alpha("alpha", {settings: {
    name: "alpha",
}});
const beta = new objectref.referencedfirst.Beta("beta", {alphaRef: {
    alpha: alpha,
}});
