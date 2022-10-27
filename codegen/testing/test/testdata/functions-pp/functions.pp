encoded = toBase64("haha business")

decoded = fromBase64(encoded)

joined = join("-", [encoded, decoded, "2"])

resource bucket "aws:s3:Bucket" {

}

encoded2 = toBase64(bucket.id)

decoded2 = fromBase64(bucket.id)
