import pulumi
import pulumi_typeddict as typeddict

main = typeddict.ExampleComponent("main",
    my_type={
        "string_prop": "hello",
        "nested_prop": {
            "nested_string_prop": "world",
            "nested_number_prop": "123",
        },
    },
    external_input=aws.s3.BucketWebsiteArgs(
        index_document="index.html",
    ))
