// Copyright 2016-2017, Pulumi Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

