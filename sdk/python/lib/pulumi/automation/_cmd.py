# Copyright 2016-2021, Pulumi Corporation.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

from __future__ import annotations
import os
import subprocess
import tempfile
import threading
import urllib.request
from typing import Any, Callable, Dict, List, Mapping, Optional

from semver import VersionInfo

from .._version import version as sdk_version
from ._env import _SKIP_VERSION_CHECK_VAR
from ._minimum_version import _MINIMUM_VERSION
from .errors import InvalidVersionError, create_command_error

OnOutput = Callable[[str], Any]


class CommandResult:
    def __init__(self, stdout: str, stderr: str, code: int) -> None:
        self.stdout = stdout
        self.stderr = stderr
        self.code = code

    def __repr__(self):
        return f"CommandResult(stdout={self.stdout!r}, stderr={self.stderr!r}, code={self.code!r})"

    def __str__(self) -> str:
        return f"\n code: {self.code}\n stdout: {self.stdout}\n stderr: {self.stderr}"


class PulumiCommand:
    """
    PulumiCommand manages the Pulumi CLI. It can be used to to install the CLI and run commands.
    """

    command: str
    version: Optional[VersionInfo]

    def __init__(
        self,
        root: Optional[str] = None,
        version: Optional[VersionInfo] = None,
        skip_version_check: bool = False,
    ):
        """
        Creates a new PulumiCommand.

        :param root: The directory where to look for the Pulumi installation. Defaults to running `pulumi` from $PATH.
        :param version: The minimum version of the Pulumi CLI to use and validates that it is compatbile with this version.
        :param skip_version_check: If true the version validation will be skipped. The env variable
                `PULUMI_AUTOMATION_API_SKIP_VERSION_CHECK` also disable this check, and takes precendence. If it is set it
                is not possible to re-enable the validation even if `skip_version_check` is `True`.
        """
        self.command = os.path.join(root, "bin", "pulumi") if root else "pulumi"
        min_version = _MINIMUM_VERSION
        if version and version.compare(min_version) > 0:
            min_version = version
        current_version = (
            subprocess.check_output([self.command, "version"]).decode("utf-8").strip()
        )
        if current_version.startswith("v"):
            current_version = current_version[1:]
        opt_out = skip_version_check or os.getenv(_SKIP_VERSION_CHECK_VAR) is not None
        self.version = _parse_and_validate_pulumi_version(
            min_version=min_version,
            current_version=current_version,
            opt_out=opt_out,
        )

    @classmethod
    def install(
        cls,
        root: Optional[str] = None,
        version: Optional[VersionInfo] = None,
        skip_version_check: bool = False,
    ) -> PulumiCommand:
        """
        Downloads and installs the Pulumi CLI.  By default the CLI version
        matching the current SDK release is installed in
        $HOME/.pulumi/versions/$VERSION. Set `root` to specify a
        different directory, and `version` to install a custom version.

        :param root: The root directory to install the CLI to. Defaults to `~/.pulumi/versions/<version>`
        :param version: The version of the CLI to install. Defaults to the version matching the SDK version.
        :skip_version_check: If true, the version validation will be skipped.
               See parameter `skip_version_check` in `__init__`."""
        if not version:
            version = sdk_version
        if not root:
            root = os.path.join(
                os.path.expanduser("~"), ".pulumi", "versions", str(version)
            )

        try:
            return PulumiCommand(
                root=root, version=version, skip_version_check=skip_version_check
            )
        except Exception:  # noqa: BLE001 catch blind exception
            pass  # Ignore

        if os.name == "nt":
            cls._install_windows(root, version)
        else:
            cls._install_posix(root, version)

        return PulumiCommand(
            root=root, version=version, skip_version_check=skip_version_check
        )

    @classmethod
    def _install_windows(cls, root: str, version: VersionInfo):
        # TODO: Once we're on python 3.12 we can use a `with` context manager with `delete_on_close=False` and `delete=True` here
        script = tempfile.NamedTemporaryFile(delete=False, suffix=".ps1")
        try:
            _download_to_file("https://get.pulumi.com/install.ps1", script.name)
            script.close()  # The file was opened for writing, so we need to close it before executing it.
            command = "powershell.exe"
            sys_root = os.getenv("SystemRoot")
            if sys_root:
                command = os.path.join(
                    sys_root,
                    "System32",
                    "WindowsPowerShell",
                    "v1.0",
                    "powershell.exe",
                )
            subprocess.check_output(
                [
                    command,
                    "-NoProfile",
                    "-InputFormat",
                    "None",
                    "-ExecutionPolicy",
                    "Bypass",
                    "-File",
                    script.name,
                    "-NoEditPath",
                    "-InstallRoot",
                    root,
                    "-Version",
                    str(version),
                ],
                stderr=subprocess.STDOUT,
            )
        finally:
            os.remove(script.name)

    @classmethod
    def _install_posix(cls, root: str, version: VersionInfo):
        # TODO: Once we're on python 3.12 we can use a `with` context manager with `delete_on_close=False` and `delete=True` here
        script = tempfile.NamedTemporaryFile(delete=False)
        try:
            _download_to_file("https://get.pulumi.com/install.sh", script.name)
            os.chmod(script.name, 0o700)
            script.close()  # The file was opened for writing, so we need to close it before executing it.
            subprocess.check_output(
                [
                    script.name,
                    "--no-edit-path",
                    "--install-root",
                    root,
                    "--version",
                    str(version),
                ],
                stderr=subprocess.STDOUT,
            )
        finally:
            os.remove(script.name)

    def run(
        self,
        args: List[str],
        cwd: str,
        additional_env: Mapping[str, str],
        on_output: Optional[OnOutput] = None,
        on_error: Optional[OnOutput] = None,
    ) -> CommandResult:
        """
        Runs a Pulumi command, returning a CommandResult. If the command fails, a CommandError is raised.

        :param args: The arguments to pass to the Pulumi CLI, for example `["stack", "ls"]`.
        :param cwd: The working directory to run the command in.
        :param additional_env: Additional environment variables to set when running the command.
        :param on_output: A callback to invoke when the command outputs stdout data.
        :param on_error: A callback to invoke when the command outputs stderr data.
        """

        # All commands should be run in non-interactive mode.
        # This causes commands to fail rather than prompting for input (and thus hanging indefinitely).
        if "--non-interactive" not in args:
            args.append("--non-interactive")
        env = {
            **os.environ,
            **additional_env,
            "PULUMI_AUTOMATION_API": "true",
        }
        if os.path.isabs(self.command):
            env = _fixup_path(env, os.path.dirname(self.command))
        cmd = [self.command]
        cmd.extend(args)

        stdout_chunks: List[str] = []
        stderr_chunks: List[str] = []

        def consumer(stream, callback, chunks):
            for line in iter(stream.readline, ""):
                stripped = line.rstrip()
                if callback:
                    callback(stripped)
                chunks.append(stripped)
            stream.close()

        with subprocess.Popen(
            cmd,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            cwd=cwd,
            env=env,
            encoding="utf-8",
        ) as process:
            assert process.stdout is not None
            assert process.stderr is not None

            stdout = threading.Thread(
                target=consumer, args=(process.stdout, on_output, stdout_chunks)
            )
            stderr = threading.Thread(
                target=consumer, args=(process.stderr, on_error, stderr_chunks)
            )

            stdout.start()
            stderr.start()

            stdout.join()
            stderr.join()

            process.wait()
            code = process.returncode

        result = CommandResult(
            stderr="\n".join(stderr_chunks), stdout="\n".join(stdout_chunks), code=code
        )
        if code != 0:
            raise create_command_error(result)

        return result


def _download_to_file(url: str, path: str):
    with urllib.request.urlopen(url) as response, open(path, "wb") as out_file:
        data = response.read()
        out_file.write(data)


def _fixup_path(env: Dict[str, str], pulumiBin: str) -> Dict[str, str]:
    """
    Fixup path so that we prioritize up the bundled plugins next to the pulumi binary.
    """
    new_env = dict(env)
    new_env["PATH"] = os.pathsep.join([pulumiBin, env["PATH"]])
    return new_env


def _parse_and_validate_pulumi_version(
    min_version: VersionInfo, current_version: str, opt_out: bool
) -> Optional[VersionInfo]:
    """
    Parse and return a version. An error is raised if the version is not
    valid. If *current_version* is not a valid version but *opt_out* is true,
    *None* is returned.
    """
    try:
        version: Optional[VersionInfo] = VersionInfo.parse(current_version)
    except ValueError:
        version = None
    if opt_out:
        return version
    if version is None:
        raise InvalidVersionError(
            f"Could not parse the Pulumi CLI version. This is probably an internal error. "
            f"If you are sure you have the correct version, set {_SKIP_VERSION_CHECK_VAR}=true."
        )
    if min_version.major < version.major:
        raise InvalidVersionError(
            f"Major version mismatch. You are using Pulumi CLI version {version} with "
            f"Automation SDK v{min_version.major}. Please update the SDK."
        )
    if min_version.compare(version) == 1:
        raise InvalidVersionError(
            f"Minimum version requirement failed. The minimum CLI version requirement is "
            f"{min_version}, your current CLI version is {version}. "
            f"Please update the Pulumi CLI."
        )
    return version
