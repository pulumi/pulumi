resource "main" "typeddict:index:ExampleComponent" {
    myType = {
        stringProp = "hello",
        nestedProp = {
            nestedStringProp = "world",
            nestedNumberProp = 123
        }
    }
    externalInput = {
		indexDocument = "index.html"
	}
}
