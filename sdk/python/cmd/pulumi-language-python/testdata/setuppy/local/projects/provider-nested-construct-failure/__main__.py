import pulumi
import pulumi_nested_failure as nested_failure

outer = nested_failure.Wrapper("outer")
