import * as pulumi from "@pulumi/pulumi";

const config = new pulumi.Config();
const names = config.requireObject<Array<string>>("names");
const tags = config.requireObject<Record<string, string>>("tags");
export const greetings = names.map(name => (`Hello, ${name}!`));
export const numbered = names.map((v, k) => [k, v] as const).map(([i, name]) => (`${i}-${name}`));
export const tagList = Object.entries(tags).sort().map(([k, v]) => (`${k}=${v}`));
export const greetingMap = names.reduce((__obj, name) => ({ ...__obj, [name]: `Hello, ${name}!` }), {});
export const filteredList = names.filter(name => name != "b").map(name => (name));
export const filteredMap = names.filter(name => name != "b").reduce((__obj, name) => ({ ...__obj, [name]: `Hello, ${name}!` }), {});
