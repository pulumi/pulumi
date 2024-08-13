import pulumi

ref = pulumi.StackReference("ref")
ref.outputs["bucket"].apply(lambda bucket: pulumi.log.info(f"Bucket: {bucket}"))
