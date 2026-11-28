import * as example from "@pulumi/example";

const widget = new example.Widget("widget");
new example.Gadget("gadget", { widgetRef: { widget } });
