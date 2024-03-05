encoded = toBase64("haha business")

decoded = fromBase64(encoded)

joined = join("-", [encoded, decoded, "2"])

# tests that we initialize "var, err" with ":=" first, then "=" subsequently (Go specific)
zone = invoke("aws:index:getAvailabilityZones", {})
zone2 = invoke("aws:index:getAvailabilityZones", {})

resource bucket "aws:s3:Bucket" { }

encoded2 = toBase64(bucket.id)

decoded2 = fromBase64(bucket.id)

secretValue = secret("hello")
plainValue = unsecret(secretValue)

currentStack = stack()
currentProject = project()
workingDirectory = cwd()
fileMimeType = mimeType("./base64.txt")

# using the filebase64 function
resource first "aws:s3:BucketObject" {
	bucket = bucket.id
	source = stringAsset(filebase64("./base64.txt"))
	contentType = fileMimeType
	tags = {
	    "stack" = currentStack
        "project" = currentProject
        "cwd" = workingDirectory
	}
}

# using the filebase64sha256 function
resource second "aws:s3:BucketObject" {
	bucket = bucket.id
	source = stringAsset(filebase64sha256("./base64.txt"))
}

# using the sha1 function
resource third "aws:s3:BucketObject" {
    bucket = bucket.id
    source = stringAsset(sha1("content"))
}
