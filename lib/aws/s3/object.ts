// Copyright 2017 Pulumi, Inc. All rights reserved.

import {Bucket} from "./bucket";
import {asset, Resource} from "@coconut/coconut";

// Object represents an Amazon Simple Storage Service (S3) object (key/value blob).
export class Object
        extends Resource
        implements ObjectProperties {
    public readonly key: string;         // the object's unique key.
    public readonly bucket: Bucket;      // the bucket this object belongs to.
    public readonly source: asset.Asset; // the source of this object's contents.
    public readonly uri: string;         // the s3:// URI for this object.

    constructor(key: string, args: ObjectProperties) {
        super();
        this.key = key;
        this.bucket = args.bucket;
        this.source = args.source;
    }

    // fromFile creates an object from a path to a file local to the deployment machine.
    public static fromFile(key: string, bucket: Bucket, path: string): Object {
        return new Object(key, {
            bucket: bucket,
            source: new asset.File(path),
        });
    }

    // fromObject copies an existing S3 object to another one.
    public static fromObject(key: string, bucket: Bucket, other: Object): Object {
        // TODO: support reading properties.
        return new Object(key, {
            bucket: bucket,
            source: new asset.Remote(other.uri),
        });
    }

    // fromString creates an object out of an in-memory string or byte array.
    // TODO: support blobs too once they are supported.
    public static fromString(key: string, bucket: Bucket, text: string): Object {
        return new Object(key, {
            bucket: bucket,
            source: new asset.String(text),
        });
    }

    // fromURI creates an object from a given URI string.  This URI's scheme dictates how the provider will fetch the
    // object's contents: file:// specifies a local file (file://); http:// and https:// specify HTTP and HTTPS,
    // respectively; and, s3:// permits you do access another existing S3 object by its URI.
    public static fromURI(key: string, bucket: Bucket, uri: string): Object {
        // TODO: the URI is going to be blank until pulumi/coconut#90 is addressed.
        return new Object(key, {
            bucket: bucket,
            source: new asset.Remote(uri),
        });
    }
}

export interface ObjectProperties {
    readonly bucket: Bucket;      // the bucket this object belongs to.
    readonly source: asset.Asset; // the source of content for this object.
}

