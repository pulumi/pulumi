resource Other "other:index:Thing" {
	idea = "Support Third Party"
}

resource Question "other:module:Object" {
    answer = 42
}

resource Provider "pulumi:providers:Provider" {
	username: "foo"
}
