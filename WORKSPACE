workspace(name = "pulumi")

# Dependencies

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "bazel_skylib",
    sha256 = "cd55a062e763b9349921f0f5db8c3933288dc8ba4f76dd9416aac68acee3cb94",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-skylib/releases/download/1.5.0/bazel-skylib-1.5.0.tar.gz",
        "https://github.com/bazelbuild/bazel-skylib/releases/download/1.5.0/bazel-skylib-1.5.0.tar.gz",
    ],
)

http_archive(
    name = "io_tweag_rules_nixpkgs",
    sha256 = "480df4a7777a5e3ee7a755ab38d18ecb3ddb7b2e2435f24ad2037c1b084faa65",
    strip_prefix = "rules_nixpkgs-126e9f66b833337be2c35103ce46ab66b4e44799",
    urls = ["https://github.com/tweag/rules_nixpkgs/archive/126e9f66b833337be2c35103ce46ab66b4e44799.tar.gz"],
)

load(
    "@io_tweag_rules_nixpkgs//nixpkgs:repositories.bzl",
    "rules_nixpkgs_dependencies",
)

rules_nixpkgs_dependencies()

http_archive(
    name = "bazel_gazelle",
    sha256 = "32938bda16e6700063035479063d9d24c60eda8d79fd4739563f50d331cb3209",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.35.0/bazel-gazelle-v0.35.0.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.35.0/bazel-gazelle-v0.35.0.tar.gz",
    ],
)

http_archive(
    name = "com_google_protobuf",
    sha256 = "532294fb03c081e8d856c1a51358fe9d5f750e12bdd09c2d31e8d2253d27005a",
    strip_prefix = "protobuf-de5e7b6b8e71cde8e63270fdd937e31a06953cc8",
    urls = [
        "https://github.com/protocolbuffers/protobuf/archive/de5e7b6b8e71cde8e63270fdd937e31a06953cc8.tar.gz",
    ],
)

http_archive(
    name = "zlib",
    build_file = "@com_google_protobuf//:third_party/zlib.BUILD",
    sha256 = "629380c90a77b964d896ed37163f5c3a34f6e6d897311f1df2a7016355c45eff",
    strip_prefix = "zlib-1.2.11",
    urls = ["https://github.com/madler/zlib/archive/v1.2.11.tar.gz"],
)

load("@com_google_protobuf//:protobuf_deps.bzl", "protobuf_deps")

protobuf_deps()

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "80a98277ad1311dacd837f9b16db62887702e9f1d1c4c9f796d0121a46c8e184",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.46.0/rules_go-v0.46.0.zip",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.46.0/rules_go-v0.46.0.zip",
    ],
)

http_archive(
    name = "rules_python",
    sha256 = "c68bdc4fbec25de5b5493b8819cfc877c4ea299c0dcb15c244c5a00208cde311",
    strip_prefix = "rules_python-0.31.0",
    url = "https://github.com/bazelbuild/rules_python/releases/download/0.31.0/rules_python-0.31.0.tar.gz",
)

load("@rules_python//python:repositories.bzl", "py_repositories")

py_repositories()

# FOO

load("@//bazel/repo:gazelle.bzl", "setup_gazelle")
load("@//bazel/repo:go.bzl", "setup_go", "go_deps")
load("@//bazel/repo:nixpkgs.bzl", "setup_nixpkgs")
load("@//bazel/repo:python.bzl", "setup_python")

setup_nixpkgs()

# gazelle:repository_macro bazel/repo/go.bzl%go_deps
go_deps()
setup_go()

setup_python()

setup_gazelle()
