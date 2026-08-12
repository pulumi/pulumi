resource "myRes" "enum:index:Res" {
    intEnum = 1
    stringEnum = "one"
}

resource "mySink" "extenumref:index:Sink" {
    stringEnum = "two"
}
