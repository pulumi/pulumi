resource Other "other:index:Thing" {
	idea = "Support Third Party"
}

resource Question "other:module:Object" {
    answer = 42
}

resource Question2 "other:module/sub:Object" {
    answer = 24
}

resource Provider "pulumi:providers:other" {
   objectProp = {
        prop1 = "foo"
        prop2 = "bar"
        prop3 = "fizz"
   }
}
