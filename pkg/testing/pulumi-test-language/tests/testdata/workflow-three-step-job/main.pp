job "build" {
  input_type = {
    input = string
  }
  expr = third

  step "first" {
    expr = "input"
  }

  step "second" {
    depends_on = ["first"]
    expr       = "input"
  }

  step "third" {
    depends_on = ["second"]
    expr       = "input"
  }
}
