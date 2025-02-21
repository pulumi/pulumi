import io
import os
import pathlib
import platform
import requests
import sys
import shutil
import subprocess
import tarfile
import tempfile
from typing import Callable, Optional
import zipfile


class Config:
    """Config collects configuration information necessary to execute build
    tasks."""

    def __init__(
        self,
        generated_dir: str,
        tools_dir: str,
        protoc_version: str,
        protoc_platforms: dict[str, str],
        protoc_gen_doc_version: str,
        protoc_gen_doc_platforms: dict[str, str],
        proto_path: str,
        proto_files: dict[str, str],
        proto_template: str,
    ) -> None:
        """Constructs a new configuration object.

        :param generated_dir:
            The directory where generated files should be written. This
            directory should not be checked into source control.
        :param tools_dir:
            The directory where build tools should be installed. This directory
            should not be checked into source control.
        :param protoc_version:
            The version of `protoc`, the Protobuf compiler, to use for build
            tasks that require it.
        :param protoc_platforms:
            A mapping from platform keys to archive keys for `protoc` releases.
        :param protoc_gen_doc_version:
            The version of `protoc-gen-doc`, the Protobuf documentation generator,
            to use for build tasks that require it.
        :param protoc_gen_doc_platforms:
            A mapping from platform keys to archive keys for `protoc-gen-doc`
            releases.
        :param proto_path:
            The path to the directory containing Protobuf files.
        :param proto_files:
            A mapping from human-readable file titles to Protobuf file names.
            Each Protobuf file will yield a separate Markdown file containing
            the contents of running `protoc-gen-doc` on the file. The file's
            H1 title will be the human-readable title provided.
        :param proto_template:
            A path to a template file that will be passed to `protoc-gen-doc` in
            order to generate Markdown for each Protobuf file. The template file
            may contain a single `{{ .TitleFromPython }}` token that will be
            replaced with the human-readable title for the Protobuf file, as
            well as the standard `protoc-gen-doc` template tokens.
        """
        self.generated_dir = pathlib.Path(generated_dir).resolve()
        self.tools_dir = pathlib.Path(tools_dir).resolve()

        platform_key = get_platform_key()

        self.protoc_version = protoc_version
        self.protoc_platforms = protoc_platforms
        self.protoc_url = f"https://github.com/protocolbuffers/protobuf/releases/download/v{self.protoc_version}/protoc-{self.protoc_version}-{self.protoc_platforms[platform_key]}.zip"

        self.protoc_gen_doc_version = protoc_gen_doc_version
        self.protoc_gen_doc_platforms = protoc_gen_doc_platforms
        self.protoc_gen_doc_url = f"https://github.com/pseudomuto/protoc-gen-doc/releases/download/v{self.protoc_gen_doc_version}/protoc-gen-doc_{self.protoc_gen_doc_version}_{self.protoc_gen_doc_platforms[platform_key]}.tar.gz"

        self.proto_path = pathlib.Path(proto_path).resolve()
        self.proto_files = {
            title: self.proto_path / file for title, file in proto_files.items()
        }
        self.proto_template = pathlib.Path(proto_template).resolve()

        self.proto_generated_dir = self.generated_dir / "proto"
        self.proto_tools_dir = self.tools_dir / "proto"
        self.protoc = self.proto_tools_dir / "protoc"
        self.protoc_gen_doc = self.proto_tools_dir / "protoc-gen-doc"


def get_platform_key() -> str:
    """Returns a key representing the current platform for use in configuration."""

    uname = platform.uname()
    platform_key = f"{uname.system}_{uname.machine}"
    return platform_key


def ensure_proto(config: Config) -> None:
    """Ensures that Protobuf tools such as `protoc` and `protoc-gen-doc` are
    installed."""

    shutil.rmtree(config.proto_tools_dir, ignore_errors=True)
    os.makedirs(config.proto_tools_dir, exist_ok=True)

    with tempfile.TemporaryDirectory() as tmpdir:
        with requests.get(
            config.protoc_url, stream=True
        ) as protoc_response, zipfile.ZipFile(
            io.BytesIO(protoc_response.content)
        ) as protoc_zip:
            protoc_zip.extractall(path=tmpdir)

        shutil.move(f"{tmpdir}/bin/protoc", config.protoc)
        shutil.move(f"{tmpdir}/include", config.proto_tools_dir)
        os.chmod(config.protoc, 0o755)

        with requests.get(
            config.protoc_gen_doc_url, stream=True
        ) as protoc_gen_doc_response, tarfile.open(
            fileobj=protoc_gen_doc_response.raw, mode="r:gz"
        ) as protoc_gen_doc_tar:
            protoc_gen_doc_tar.extractall(path=tmpdir)

        shutil.move(f"{tmpdir}/protoc-gen-doc", config.protoc_gen_doc)
        os.chmod(config.protoc_gen_doc, 0o755)


def generate_proto(config: Config) -> None:
    """Generates Markdown documentation for each configured Protobuf file."""

    shutil.rmtree(config.proto_generated_dir, ignore_errors=True)
    os.makedirs(config.proto_generated_dir, exist_ok=True)

    template = config.proto_template.read_text(encoding="utf-8")
    for title, file in config.proto_files.items():
        with tempfile.NamedTemporaryFile() as tmpfile:
            tmpfile.write(
                template.replace("{{ .TitleFromPython }}", title).encode("utf-8")
            )
            tmpfile.flush()

            args = [
                config.protoc,
                f"--plugin=protoc-gen-doc={config.protoc_gen_doc}",
                f"--doc_out={config.proto_generated_dir}",
                f"--doc_opt={tmpfile.name},{file.stem}.md",
                f"--proto_path={config.proto_path}",
                f"{config.proto_path/file}",
            ]
            subprocess.run(args, check=True)


TARGETS: dict[str, Callable[[Config], None]] = {
    "ensure-proto": ensure_proto,
    "generate-proto": generate_proto,
}


def main():
    """The main program entry point."""

    if len(sys.argv) < 2:
        usage()

    target = sys.argv[1]
    if target not in TARGETS:
        usage(f"unknown target '{target}'")

    config = Config(
        generated_dir="_generated",
        tools_dir="_tools",
        protoc_version="25.1",
        protoc_platforms={
            "Linux_x86_64": "linux-x86_64",
            "Darwin_arm64": "osx-aarch_64",
        },
        protoc_gen_doc_version="1.5.1",
        protoc_gen_doc_platforms={
            "Linux_x86_64": "linux_amd64",
            "Darwin_arm64": "darwin_arm64",
        },
        proto_path="../proto",
        proto_files={
            "Aliasing": "pulumi/alias.proto",
            "Analysis": "pulumi/analyzer.proto",
            "Callbacks": "pulumi/callback.proto",
            "Conversion mappings": "pulumi/codegen/mapper.proto",
            "Engine services": "pulumi/engine.proto",
            "Errors": "pulumi/errors.proto",
            "Language hosts": "pulumi/language.proto",
            "Plugins": "pulumi/plugin.proto",
            "Program conversion": "pulumi/converter.proto",
            "Providers": "pulumi/provider.proto",
            "Resource registration": "pulumi/resource.proto",
            "Schema loading": "pulumi/codegen/loader.proto",
            "Source positions": "pulumi/source.proto",
        },
        proto_template="references/proto.md.tmpl",
    )

    TARGETS[target](config)


def usage(error: Optional[str] = None):
    """Prints usage information and exits with a non-zero status."""

    if error:
        print(f"Error: {error}")

    print("Usage: python make.py [target]")
    print()
    print("Targets:")
    for target in sorted(TARGETS):
        print(f"  {target}")

    sys.exit(1)


if __name__ == "__main__":
    main()
