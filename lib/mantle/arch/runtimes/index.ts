// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// The available language runtimes.
export const nodejs = "nodejs";
export const python = "python";

export let ext: {[lang: string]: string} = {
    nodejs: ".js",
    python: ".py",
};

