job "build" {
  input_type = {
    input = bool
  }
  expr       = !args.input
}
