resource "catExample" "discriminated-union:index:Example" {
  pet = { petType = "cat", meow = "meow" }
  pets = [{ petType = "cat", meow = "purr" }]
}

resource "dogExample" "discriminated-union:index:Example" {
  pet = { petType = "dog", bark = "woof" }
  pets = [{ petType = "dog", bark = "bark" }, { petType = "cat", meow = "hiss" }]
}
