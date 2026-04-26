import pulumi
import pulumi_keywords as keywords

first_resource = keywords.SomeResource("firstResource",
    builtins="builtins",
    lambda_="lambda",
    property="property")
second_resource = keywords.SomeResource("secondResource",
    builtins=first_resource.builtins,
    lambda_=first_resource.lambda_,
    property=first_resource.property)
lambda_module_resource = keywords.lambda_.SomeResource("lambdaModuleResource",
    builtins="builtins",
    lambda_="lambda",
    property="property")
lambda_resource = keywords.Lambda("lambdaResource",
    builtins="builtins",
    lambda_="lambda",
    property="property")
