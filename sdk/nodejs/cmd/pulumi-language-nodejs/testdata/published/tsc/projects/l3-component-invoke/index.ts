import * as pulumi from "@pulumi/pulumi";
import * as config from "@pulumi/config";
import { InvokeComponent } from "./invokeComponent";

const prov = new config.Provider("prov", {name: "my config"});
const myComponent = new InvokeComponent("myComponent", {
    providers: [prov],
});
