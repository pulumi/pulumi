resource provider "pulumi:providers:unknown" { }

resource main "unknown:index:main" {
    first = "hello"
    second = {
        foo = "bar"
    }
}

resource fromModule "unknown:eks:example" {
   options { range = 10 }
   associatedMain = main.id
}

output "mainId" {
    value = main.id
}

output "values" {
    value = fromModule.values.first
}