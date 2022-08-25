resource Other "other:index:Thing" {
	idea = "Support Third Party"
}

resource Question "other:module:Object" {
    answer = 42
}

<<<<<<< Updated upstream
resource Provider "pulumi:providers:thirdpartyprovider" {
=======
resource Provider "pulumi:providers:Provider" {
>>>>>>> Stashed changes
	username: "foo"
}
