// Copyright 2025, Pulumi Corporation.  All rights reserved.

import * as fs from 'fs';
import * as provider from '@pulumi/provider';

const comp = new provider.MyComponent("comp")

comp.urn.apply(_ => {
    fs.writeFileSync('ready.txt', 'ready');
})

export const wait = new Promise(resolve => setTimeout(resolve, 5 * 60 * 1000));
