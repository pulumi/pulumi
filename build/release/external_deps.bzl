"""Module extension for downloading external release binaries.

Downloads pre-built binaries for pulumi-watch, pulumi-language-dotnet,
pulumi-language-java, and pulumi-language-yaml for all release platforms.
"""

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

# pulumi-watch binaries from pulumi/watchutil-rs.
_WATCH_VERSION = "v0.1.4"
_WATCH_BINARIES = {
    "linux_amd64": struct(
        triple = "x86_64-unknown-linux-gnu",
        ext = "tar.gz",
        binary = "pulumi-watch",
    ),
    "linux_arm64": struct(
        triple = "aarch64-unknown-linux-gnu",
        ext = "tar.gz",
        binary = "pulumi-watch",
    ),
    "darwin_amd64": struct(
        triple = "x86_64-apple-darwin",
        ext = "tar.gz",
        binary = "pulumi-watch",
    ),
    "darwin_arm64": struct(
        triple = "aarch64-apple-darwin",
        ext = "tar.gz",
        binary = "pulumi-watch",
    ),
    "windows_amd64": struct(
        triple = "x86_64-pc-windows-msvc",
        ext = "zip",
        binary = "pulumi-watch.exe",
    ),
    # No windows_arm64 binary exists for pulumi-watch.
}

# Language provider binaries.
_LANG_PROVIDERS = {
    "dotnet": "v3.101.2",
    "java": "v1.21.2",
    "yaml": "v1.29.1",
}

# All OS/arch combos for language providers.
_LANG_PLATFORMS = [
    ("linux", "amd64"),
    ("linux", "arm64"),
    ("darwin", "amd64"),
    ("darwin", "arm64"),
    ("windows", "amd64"),
    ("windows", "arm64"),
]

def _release_deps_impl(ctx):
    # Register pulumi-watch repos.
    for platform_key, info in _WATCH_BINARIES.items():
        archive_base = "pulumi-watch-%s-%s" % (_WATCH_VERSION, info.triple)
        url = "https://github.com/pulumi/watchutil-rs/releases/download/%s/%s.%s" % (
            _WATCH_VERSION,
            archive_base,
            info.ext,
        )
        name = "pulumi_watch_%s" % platform_key

        http_archive(
            name = name,
            url = url,
            build_file_content = 'exports_files(["%s"])' % info.binary,
            strip_prefix = archive_base,
        )

    # Register language provider repos.
    for lang, version in _LANG_PROVIDERS.items():
        for goos, goarch in _LANG_PLATFORMS:
            ext = ".exe" if goos == "windows" else ""
            binary = "pulumi-language-%s%s" % (lang, ext)
            archive = "pulumi-language-%s-%s-%s-%s.tar.gz" % (lang, version, goos, goarch)
            url = "https://github.com/pulumi/pulumi-%s/releases/download/%s/%s" % (lang, version, archive)
            name = "pulumi_language_%s_%s_%s" % (lang, goos, goarch)

            http_archive(
                name = name,
                url = url,
                build_file_content = 'exports_files(["%s"])' % binary,
            )

release_deps = module_extension(
    implementation = _release_deps_impl,
)
