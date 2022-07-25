resource siteBucket "aws:s3:Bucket" { }

resource testFileAsset "aws:s3:BucketObject" {
	bucket = siteBucket.id // Reference the s3.Bucket object
	source = fileAsset("file.txt")
}

resource testStringAsset "aws:s3:BucketObject" {
	bucket = siteBucket.id // Reference the s3.Bucket object
	source = stringAsset("<h1>File contents</h1>")
}

resource testRemoteAsset "aws:s3:BucketObject" {
	bucket = siteBucket.id // Reference the s3.Bucket object
	source = remoteAsset("https://pulumi.test")
}

resource testFileArchive "aws:s3:BucketObject" {
	bucket = siteBucket.id // Reference the s3.Bucket object
	source = fileArchive("file.tar.gz")
}

resource testRemoteArchive "aws:s3:BucketObject" {
	bucket = siteBucket.id // Reference the s3.Bucket object
	source = remoteArchive("https://pulumi.test/foo.tar.gz")
}

resource testAssetArchive "aws:s3:BucketObject" {
	bucket = siteBucket.id // Reference the s3.Bucket object
	source = assetArchive({
		"file.txt": fileAsset("file.txt")
		"string.txt": stringAsset("<h1>File contents</h1>")
		"remote.txt": remoteAsset("https://pulumi.test")
		"file.tar": fileArchive("file.tar.gz")
		"remote.tar": remoteArchive("https://pulumi.test/foo.tar.gz")
		".nestedDir": assetArchive({
			"file.txt": fileAsset("file.txt")
			"string.txt": stringAsset("<h1>File contents</h1>")
			"remote.txt": remoteAsset("https://pulumi.test")
			"file.tar": fileArchive("file.tar.gz")
			"remote.tar": remoteArchive("https://pulumi.test/foo.tar.gz")
		})
	})
}