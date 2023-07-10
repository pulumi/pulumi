import pulumi
import pulumi_aws as aws

config = pulumi.Config()
# Number of AZs to cover in a given region
az_count = config.get("azCount")
if az_count is None:
    az_count = "10"
buckets_per_availability_zone = []
for range in [{"value": i} for i in range(0, int(az_count))]:
    buckets_per_availability_zone.append(aws.s3.Bucket(f"bucketsPerAvailabilityZone-{range['value']}", website=aws.s3.BucketWebsiteArgs(
        index_document="index.html",
    )))
