import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

const _class = new simple.Resource("class", {value: true});
const _export = new simple.Resource("export", {value: true});
const mod = new simple.Resource("mod", {value: true});
const _import = new simple.Resource("import", {value: true});
const object = new simple.Resource("object", {value: true});
const self = new simple.Resource("self", {value: true});
const _this = new simple.Resource("this", {value: true});
