import * as pulumi from "@pulumi/pulumi";
import * as module_format from "@pulumi/module-format";
import * as names from "@pulumi/names";

const config = new pulumi.Config();
const names2 = config.getBoolean("names") || true;
const Names = config.getBoolean("Names") || true;
const mod = config.get("mod") || "module";
const Mod = config.get("Mod") || "format";
const namesResource = new names.mod.Res("namesResource", {value: names2});
const modResource = new module_format.mod.Resource("modResource", {text: `${mod}-${Mod}`});
export const namesResourceVal = namesResource.value;
export const modResourceText = modResource.text;
export const nameVariables = names2 && Names;
export const modVariables = `${mod}-${Mod}`;
