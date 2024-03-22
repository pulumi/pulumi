def _directory_repo_impl(repository_ctx):
  repository_ctx.file(
    "WORKSPACE.bazel",
    content = """workspace(name = "{repo_name}")""".format(
      repo_name = repository_ctx.attr.name,
    )
  )
  repository_ctx.file(
    "BUILD.bazel",
    content = "",
  )

  result = repository_ctx.execute([
    "find", "%s/%s" % (repository_ctx.workspace_root, repository_ctx.attr.directory),
    "-name", "BUILD.bazel",
  ])
  if result.return_code != 0:
    fail(result.stderr)

  prefix = "{workspace_root}/{directory}/".format(
    workspace_root = str(repository_ctx.workspace_root),
    directory = repository_ctx.attr.directory,
  )
  packages = [
    build_file.removeprefix(prefix).removesuffix("BUILD.bazel").removesuffix("/")
    for build_file in result.stdout.split("\n")
  ]

  for package in packages:
    if package == "":
      continue
    repository_ctx.file(
      package + "/BUILD.bazel",
      content = """
alias(
  name = "go_default_library",
  actual = "@pulumi//{directory}/{actual}",
  visibility = ["//visibility:public"],
)
""".format(directory = repository_ctx.attr.directory, actual = package),
    )

directory_repo = repository_rule(
  _directory_repo_impl,
  attrs = {
    "directory": attr.string(),
  },
)
