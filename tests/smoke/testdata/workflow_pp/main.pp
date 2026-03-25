workflow "main" {
  job "build" {
    step "prepare" {
      expr = "prepare-ok"
    }
    step "test" {
      command = "printf 'test-ok'"
      depends_on = ["prepare"]
    }
  }
}
