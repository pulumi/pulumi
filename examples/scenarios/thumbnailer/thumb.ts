// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as mu from "mu";

export class Thumbnailer extends mu.Stack {
    private source: mu.x.Bucket; // the source to monitor for images.
    private dest: mu.x.Bucket;   // the destination to store thumbnails in.

    constructor(source: mu.x.Bucket, dest: mu.x.Bucket) {
        super();
        this.source = source;
        this.dest = dest;
        this.source.onObjectCreated(async (event) => {
            let obj = await event.GetObject();
            let thumb = await gm(obj.Data).thumbnail();
            await this.dest.PutObject(thumb);
        });
    }
}

