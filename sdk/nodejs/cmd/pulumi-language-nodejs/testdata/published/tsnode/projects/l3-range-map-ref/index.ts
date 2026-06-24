import * as pulumi from "@pulumi/pulumi";
import * as nestedobject from "@pulumi/nestedobject";

const config = new pulumi.Config();
const itemMap = config.requireObject<Record<string, string>>("itemMap");
const mapResource: {[key: string]: nestedobject.Target} = {};
for (const range of Object.entries(itemMap).sort().map(([k, v]) => ({key: k, value: v}))) {
    mapResource[range.key] = new nestedobject.Target(`mapResource-${range.key}`, {name: `${range.key}=${range.value}`});
}
const mapTarget = new nestedobject.Target("mapTarget", {name: pulumi.interpolate`${mapResource.k1.name}+`});
