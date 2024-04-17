import pulumi
import pulumi_typeddict as typeddict

main = typeddict.ExampleComponent("main",
    my_type={
        "stringProp": "hello",
        "nestedProp": {
            "nestedStringProp": "world",
            "nestedNumberProp": "123",
        },
    },
    external_input=aws.s3.BucketWebsiteArgs(
        index_document="index.html",
    ))
