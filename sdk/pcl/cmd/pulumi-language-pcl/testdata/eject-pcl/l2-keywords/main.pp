resource "firstResource" "keywords:index:SomeResource" {
    builtins = "builtins"
    lambda = "lambda"
    property = "property"
}

resource "secondResource" "keywords:index:SomeResource" {
    builtins = firstResource.builtins
    lambda = firstResource.lambda
    property = firstResource.property
}

resource "lambdaModuleResource" "keywords:lambda:SomeResource" {
    builtins = "builtins"
    lambda = "lambda"
    property = "property"
}

resource "lambdaResource" "keywords:index:Lambda" {
    builtins = "builtins"
    lambda = "lambda"
    property = "property"
}
