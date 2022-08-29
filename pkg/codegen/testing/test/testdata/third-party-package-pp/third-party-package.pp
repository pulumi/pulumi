resource Other "other:index:Thing" {
	idea = "Support Third Party"
}

resource Question "other:module:Object" {
    answer = 42
}

resource Provider "pulumi:providers:other" {
   objectProp = {
        propStr = "foo"
        propBool = true
        propInt = 42
   }
}
