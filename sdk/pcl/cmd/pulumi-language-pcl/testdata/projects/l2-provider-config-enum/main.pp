resource "prov" "pulumi:providers:config-enum" {
    aString = "hello"
    aEnum = "two"
}

# Reference the provider's outputs - including the enum - from another resource.
resource "res" "config-enum:index:Resource" {
    options {
        provider = prov
    }

    theString = prov.aString
    theEnum = prov.aEnum
}
