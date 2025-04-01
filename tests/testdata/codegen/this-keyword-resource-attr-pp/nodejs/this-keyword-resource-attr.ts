import * as pulumi from "@pulumi/pulumi";
import { Submodule } from "./submodule";

// Make this a component/submodule so that parent references are generated in TS
const test = new Submodule("test", {name: "fakename"});
export const foo = test.someOutput;
