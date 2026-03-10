import * as pulumi from "@pulumi/pulumi";

const config = new pulumi.Config();
const names = config.requireObject<Array<string>>("names");
const tags = config.requireObject<Record<string, string>>("tags");
export const greetings = names.map((v, k) => [k, v]).map(([_, name]) => (`Hello, ${name}!`));
export const numbered = names.map((v, k) => [k, v]).map(([i, name]) => (`${i}-${name}`));
export const tagList = Object.entries(tags).map(([k, v]) => (`${k}=${v}`));
