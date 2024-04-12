load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies")

def setup_gazelle(name = "setup_gazelle"):
    gazelle_dependencies(go_sdk = "go_sdk")
