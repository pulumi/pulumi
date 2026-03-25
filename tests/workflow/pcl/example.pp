trigger "cron" {
  schedule = "* * * * *"
}

step "package" {
  expr = "package-ok"
}

job "bootstrap" {
  step "prepare" {
    command = "printf 'prepare-ok'"
  }
}

workflow "main" {
  trigger "cron" {
    uses = "example:cron"
    schedule = "* * * * *"
  }

  job "bootstrap" {
    uses = "bootstrap"
  }

  job "main" {
    depends_on = ["bootstrap"]

    step "prepare" {
      command = "printf 'prepare-ok'"
    }
    step "compile" {
      command = "printf 'compile-ok'"
      depends_on = ["prepare"]
    }
    step "test" {
      command = "printf 'test-ok'"
      depends_on = ["compile"]
    }
    step "skipped" {
      command = "printf 'should-not-run'"
      if = false
      depends_on = ["compile"]
    }
    step "package" {
      uses = "package"
      depends_on = ["compile", "test"]
    }
  }

  job "disabled-job" {
    if = false
    step "never" {
      command = "printf 'never'"
    }
  }
}
