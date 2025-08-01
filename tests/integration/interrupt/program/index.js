// Copyright 2025, Pulumi Corporation.  All rights reserved.

import * as fs from 'fs';

fs.writeFileSync('ready.txt', 'ready');

export const wait = new Promise(resolve => setTimeout(resolve, 5 * 60 * 1000));
