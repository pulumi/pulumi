// Copyright 2024, Pulumi Corporation.  All rights reserved.

// This program should fail with an out of memory error.

const v8 = require('node:v8')

function heapInfo() {
    const stats = v8.getHeapStatistics();
    console.log(`used: ${stats.used_heap_size / 1024 / 1024}, limit: ${stats.heap_size_limit / 1024 / 1024}`);
}

const data = []

for (let i = 0; i < 1_000_000; i++) {
    if (i % 100 === 0) heapInfo();
    data.push(new Array(1_000_000).fill('a'))
}
