job "build" {
  input_type = {
    input = string
  }
  expr = "${first} + ${third}"

  step "first" {
    expr = args.input
  }

  step "second" {
    expr = "${first} text"
  }

  step "third" {
    expr = "${second} tail"
  }
}
