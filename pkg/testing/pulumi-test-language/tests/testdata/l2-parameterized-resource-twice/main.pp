package "hipackage" {
    baseProviderName = "parameterized"
    baseProviderVersion = "1.2.3"
    parameterization {
        name = "hipackage"
        version = "2.0.0"
        value = "SGVsbG9Xb3JsZA==" // base64(utf8_bytes("HelloWorld"))
    }
}

// The resource name is based on the parameter value
resource example1 "hipackage:index:HelloWorld" { }

resource exampleComponent1 "hipackage:index:HelloWorldComponent" { }

output "parameterValue1" {
    value = example1.parameterValue
}

output "parameterValueFromComponent1" {
    value = exampleComponent1.parameterValue
}

package "byepackage" {
    baseProviderName = "parameterized"
    baseProviderVersion = "1.2.3"
    parameterization {
        name = "byepackage"
        version = "2.0.0"
        value = "R29vZGJ5ZVdvcmxk" // base64(utf8_bytes("GoodbyeWorld"))
    }
}

// The resource name is based on the parameter value
resource example2 "byepackage:index:GoodbyeWorld" { }

resource exampleComponent2 "byepackage:index:GoodbyeWorldComponent" { }

output "parameterValue2" {
    value = example2.parameterValue
}

output "parameterValueFromComponent2" {
    value = exampleComponent2.parameterValue
}