// Copyright 2017 Pulumi, Inc. All rights reserved.

// The available language runtimes.

export const NodeJS = "nodejs";
export const Python = "python";

export let ext: {[lang: string]: string} = {
    NodeJS: ".js",
    Python: ".py",
};

