trigger "cron" {
  schedule = "* * * * *"
}

step "prepare" {
  command = "printf 'prepare-ok'"
}

step "compile" {
  command = "printf 'compile-ok'"
}

step "test" {
  command = "printf 'test-ok'"
}

step "package" {
  expr = "package-ok"
}

job "bootstrap" {
  step "prepare" {
    uses = "prepare"
  }
}

job "main" {
  step "prepare" {
    uses = "prepare"
  }
  step "compile" {
    uses = "compile"
    depends_on = ["prepare"]
  }
  step "test" {
    uses = "example:test"
    depends_on = ["compile"]
  }
  step "package" {
    uses = "package"
    depends_on = ["compile", "test"]
  }
}

workflow "main" {
  trigger_ref "cron" {}
  job "bootstrap" {
    uses = "bootstrap"
  }
  job "main" {
    uses = "example:main"
    depends_on = ["bootstrap"]
  }
}
