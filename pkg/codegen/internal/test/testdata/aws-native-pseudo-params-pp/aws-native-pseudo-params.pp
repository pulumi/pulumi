awsPartition = invoke("aws-native::getPartition").partition

config trailName string {
}

config bucketName string {
}

isOrganizationsSupported = awsPartition == "aws"

resource trail "aws-native:cloudtrail:Trail" {
	s3BucketName = bucketName
	s3KeyPrefix = "Uluru"
	isLogging = true
	trailName = trailName
	enableLogFileValidation = true
	includeGlobalServiceEvents = true
	isMultiRegionTrail = true
	cloudWatchLogsLogGroupArn = invoke("aws-native::importValue", {
		name = "TrailLogGroupTestArn"
	}).value
	cloudWatchLogsRoleArn = invoke("aws-native::importValue", {
		name = "TrailLogGroupRoleTestArn"
	}).value
	kmsKeyId = invoke("aws-native::importValue", {
		name = "TrailKeyTest"
	}).value
	tags = [
		{
			key = "TagKeyIntTest",
			value = "TagValueIntTest"
		},
		{
			key = "TagKeyIntTest2",
			value = "TagValueIntTest2"
		}
	]
	snsTopicName = invoke("aws-native::importValue", {
		name = "TrailTopicTest"
	}).value
	eventSelectors = [{
		dataResources = [{
			type = "AWS::S3::Object",
			values = ["arn:${awsPartition}:s3:::"]
		}],
		includeManagementEvents = true,
		readWriteType = "All"
	}]
}

output arn {
	value = trail.arn
}

output topicArn {
	value = trail.snsTopicArn
}
