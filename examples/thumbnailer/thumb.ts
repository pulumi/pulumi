import * as mu from "mu";

export class Thumbnailer extends mu.Stack {
    private source: mu.Bucket; // the source to monitor for images.
    private dest: mu.Bucket;   // the destination to store thumbnails in.

    constructor(source: mu.Bucket, dest: mu.Bucket) {
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

