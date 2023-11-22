// Copyright 2016-2020, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import execa from "execa";
import * as fs from "fs";
import * as path from "path";
import * as semver from "semver";
import * as tmp from "tmp";
import * as util from "util";

import { createCommandError } from "./errors";

/** @internal */
export class CommandResult {
    stdout: string;
    stderr: string;
    code: number;
    err?: Error;
    constructor(stdout: string, stderr: string, code: number, err?: Error) {
        this.stdout = stdout;
        this.stderr = stderr;
        this.code = code;
        this.err = err;
    }
    toString(): string {
        let errStr = "";
        if (this.err) {
            errStr = this.err.toString();
        }
        return `code: ${this.code}\n stdout: ${this.stdout}\n stderr: ${this.stderr}\n err?: ${errStr}\n`;
    }
}

export interface PulumiOptions {
    version?: semver.SemVer;
    root?: string;
}

export class Pulumi {
    private constructor(readonly command: string, readonly version: semver.SemVer) {}

    static async get(opts?: PulumiOptions): Promise<Pulumi> {
        const command = opts?.root ? path.resolve(path.join(opts.root, "bin/pulumi")) : "pulumi";

        const { stdout } = await exec(command, ["version"]);

        const version = semver.parse(stdout) || semver.parse("3.0.0")!;
        if (opts?.version && version.compare(opts.version.toString()) < 0) {
            throw Error(`${command} version ${version} does not satisfy version ${opts.version}`);
        }
        return new Pulumi(command, version);
    }

    static async install(opts?: PulumiOptions): Promise<Pulumi> {
        try {
            return await Pulumi.get(opts);
        } catch (err) {
            // ignore
        }

        if (process.platform === "win32") {
            await Pulumi.installWindows(opts);
        } else {
            await Pulumi.installPosix(opts);
        }

        return await Pulumi.get(opts);
    }

    private static async installWindows(opts?: PulumiOptions): Promise<void> {
        const script = await writeTempFile(installWindowsScript);

        try {
            const command = process.env.SystemRoot
                ? path.join(process.env.SystemRoot, "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
                : "powershell.exe";

            const args = [
                "-NoProfile",
                "-InputFormat",
                "None",
                "-ExecutionPolicy",
                "Bypass",
                "-File",
                script.path,
            ];

            if (opts?.root) {
                args.push("-InstallRoot", opts.root);
            }
            if (opts?.version) {
                args.push("-Version", `${opts.version}`);
            }

            await exec(command, args);
        } finally {
            script.cleanup();
        }
    }

    private static async installPosix(opts?: PulumiOptions): Promise<void> {
        const script = await writeTempFile(installPosixScript);

        try {
            const args = [script.path, "--no-path"];
            if (opts?.root) {
                args.push("--install-root", opts.root);
            }
            if (opts?.version) {
                args.push("--version", `${opts.version}`);
            }

            await exec("/bin/sh", args);
        } finally {
            script.cleanup();
        }
    }

    public run(
        args: string[],
        cwd: string,
        additionalEnv: { [key: string]: string },
        onOutput?: (data: string) => void,
    ): Promise<CommandResult> {
        // all commands should be run in non-interactive mode.
        // this causes commands to fail rather than prompting for input (and thus hanging indefinitely)

        if (!args.includes("--non-interactive")) {
            args.push("--non-interactive");
        }

        return exec(this.command || "pulumi", args, cwd, additionalEnv, onOutput);
    }
}

async function exec(
    command: string,
    args: string[],
    cwd?: string,
    additionalEnv?: { [key: string]: string },
    onOutput?: (data: string) => void,
): Promise<CommandResult> {
    const unknownErrCode = -2;

    const env = additionalEnv ? { ...additionalEnv } : undefined;

    try {
        const proc = execa(command, args, { env, cwd });

        if (onOutput && proc.stdout) {
            proc.stdout!.on("data", (data: any) => {
                if (data?.toString) {
                    data = data.toString();
                }
                onOutput(data);
            });
        }

        const { stdout, stderr, exitCode } = await proc;
        const commandResult = new CommandResult(stdout, stderr, exitCode);
        if (exitCode !== 0) {
            throw createCommandError(commandResult);
        }

        return commandResult;
    } catch (err) {
        const error = err as Error;
        throw createCommandError(new CommandResult("", error.message, unknownErrCode, error));
    }
}

function writeTempFile(contents: string): Promise<{ path: string; cleanup: () => void }> {
    return new Promise<{ path: string; cleanup: () => void }>((resolve, reject) => {
        tmp.file((tmpErr, tmpPath, fd, cleanup) => {
            if (tmpErr) {
                reject(tmpErr);
            } else {
                fs.writeFile(fd, contents, (writeErr) => {
                    if (writeErr) {
                        cleanup();
                        reject(writeErr);
                    } else {
                        resolve({ path: tmpPath, cleanup });
                    }
                });
            }
        });
    });
}

const installWindowsScript = `param(
    [string]$Version
    [string]$InstallRoot=$null
	[bool]$AddToPath=$false
)

Set-StrictMode -Version Latest
$ErrorActionPreference="Stop"
$ProgressPreference="SilentlyContinue"

# Some versions of PowerShell do not support Tls1.2 out of the box, but pulumi.com requires it
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

if ($Version -eq $null -or $Version -eq "") {
    # Query pulumi.com/latest-version for the most recent release. Because this approach
    # is now used by third parties as well (e.g., GitHub Actions virtual environments),
    # changes to this API should be made with care to avoid breaking any services that
    # rely on it (and ideally be accompanied by PRs to update them accordingly). Known
    # consumers of this API include:
    #
    # * https://github.com/actions/virtual-environments
    #
    $latestVersion = (Invoke-WebRequest -UseBasicParsing https://www.pulumi.com/latest-version).Content.Trim()
    $Version = $latestVersion
}

$downloadUrl = "https://get.pulumi.com/releases/sdk/pulumi-v\${Version}-windows-x64.zip"

Write-Host "Downloading $downloadUrl"

# Download to a temp file, Expand-Archive requires that the extention of the file be "zip", so we do a bit of work here
# to generate the filename.
$tempZip = New-Item -Type File (Join-Path $env:TEMP ([System.IO.Path]::ChangeExtension(([System.IO.Path]::GetRandomFileName()), "zip")))
Invoke-WebRequest $downloadUrl -OutFile $tempZip

# Extract the zip we've downloaded. It contains a single root folder named "Pulumi" with a sub-directory named "bin"
$tempDir = New-Item -Type Directory (Join-Path $env:TEMP ([System.IO.Path]::GetRandomFileName()))

# PowerShell 5.0 added a nice Expand-Archive command, which we'll use when its present, otherwise we fallback to using .NET
if ($PSVersionTable.PSVersion.Major -ge 5) {
    Expand-Archive $tempZip $tempDir
} else {
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    [System.IO.Compression.ZipFile]::ExtractToDirectory($tempZip, $tempDir)
}

$pulumiInstallRoot = $InstallRoot
if (-not $pulumiInstallRoot) {
    # Install into %USERPROFILE%\\.pulumi\\bin by default
    $pulumiInstallRoot = (Join-Path $env:UserProfile ".pulumi")
}
$binRoot = (Join-Path $pulumiInstallRoot "bin")

Write-Host "Copying Pulumi to $binRoot"

# If we have a previous install, remove files with a pulumi prefix
if (Test-Path -Path (Join-Path $binRoot "pulumi")) {
    Get-ChildItem -Path $binRoot -File | Where-Object { $_.Name -like "pulumi*" } | ForEach-Object {
        Remove-Item $_.FullName -Force
    }
}

# Create the %USERPROFILE%\\.pulumi\\bin directory if it doesn't exist
if (-not (Test-Path -Path $binRoot -PathType Container)) {
    New-Item -Path $binRoot -ItemType Directory
}

# Our tarballs used to have a top level bin folder, so support that older
# format if we detect it. Newer tarballs just have all the binaries in
# the top level Pulumi folder.
if (Test-Path (Join-Path $tempDir (Join-Path "pulumi" "bin"))) {
    Get-ChildItem -Path (Join-Path $tempDir (Join-Path "pulumi" "bin")) -File | ForEach-Object {
        $destinationPath = Join-Path -Path $binRoot -ChildPath $_.Name
        Move-Item -Path $_.FullName -Destination $destinationPath -Force
    }
} else {
    Move-Item (Join-Path $tempDir (Join-Path "pulumi" "bin")) $binRoot
}


# Attempt to add ourselves to the $PATH, but if we can't, don't fail the overall script.
if ($AddToPath) {
	try {
		$envKey = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey("Environment", [Microsoft.Win32.RegistryKeyPermissionCheck]::ReadWriteSubTree);
		$val = $envKey.GetValue("PATH", "", [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames);
		if ($val -notlike "*\${binRoot};*") {
			$envKey.SetValue("PATH", "$binRoot;$val", [Microsoft.Win32.RegistryValueKind]::ExpandString);
			Write-Host "Added $binRoot to the \`$PATH. Changes may not be visible until after a restart."
		}
		$envKey.Close();
	} catch {
	}

	if ($env:PATH -notlike "*$binRoot*") {
		$env:PATH = "$binRoot;$env:PATH"
	}
}

# And cleanup our temp files
Remove-Item -Force $tempZip
Remove-Item -Recurse -Force $tempDir

Write-Host "Pulumi is now installed!"
Write-Host ""
Write-Host "Ensure that $binRoot is on your \`$PATH to use it."
Write-Host ""
Write-Host "Get started with Pulumi: https://www.pulumi.com/docs/quickstart"
`;

const installPosixScript = `
#!/bin/sh
set -e

RESET="\\\\033[0m"
RED="\\\\033[31;1m"
GREEN="\\\\033[32;1m"
YELLOW="\\\\033[33;1m"
BLUE="\\\\033[34;1m"
WHITE="\\\\033[37;1m"

print_unsupported_platform()
{
    >&2 say_red "error: We're sorry, but it looks like Pulumi is not supported on your platform"
    >&2 say_red "       We support 64-bit versions of Linux and macOS and are interested in supporting"
    >&2 say_red "       more platforms.  Please open an issue at https://github.com/pulumi/pulumi and"
    >&2 say_red "       let us know what platform you're using!"
}

say_green()
{
    [ -z "\${SILENT}" ] && printf "%b%s%b\\\\n" "\${GREEN}" "$1" "\${RESET}"
    return 0
}

say_red()
{
    printf "%b%s%b\\\\n" "\${RED}" "$1" "\${RESET}"
}

say_yellow()
{
    [ -z "\${SILENT}" ] && printf "%b%s%b\\\\n" "\${YELLOW}" "$1" "\${RESET}"
    return 0
}

say_blue()
{
    [ -z "\${SILENT}" ] && printf "%b%s%b\\\\n" "\${BLUE}" "$1" "\${RESET}"
    return 0
}

say_white()
{
    [ -z "\${SILENT}" ] && printf "%b%s%b\\\\n" "\${WHITE}" "$1" "\${RESET}"
    return 0
}

at_exit()
{
    # shellcheck disable=SC2181
    # https://github.com/koalaman/shellcheck/wiki/SC2181
    # Disable because we don't actually know the command we're running
    if [ "$?" -ne 0 ]; then
        >&2 say_red
        >&2 say_red "We're sorry, but it looks like something might have gone wrong during installation."
        >&2 say_red "If you need help, please join us on https://slack.pulumi.com/"
    fi
}

trap at_exit EXIT

VERSION=""
INSTALL_ROOT=""
NO_PATH=""
SILENT=""
while [ $# -gt 0 ]; do
    case "$1" in
        --version)
            if [ "$2" != "latest" ]; then
                VERSION=$2
            fi
            ;;
        --silent)
            SILENT="--silent"
            ;;
		--install-root)
			INSTALL_ROOT=$2
			;;
		--no-path)
			NO_PATH="true"
			;;
     esac
     shift
done

if [ -z "\${VERSION}" ]; then

    # Query pulumi.com/latest-version for the most recent release. Because this approach
    # is now used by third parties as well (e.g., GitHub Actions virtual environments),
    # changes to this API should be made with care to avoid breaking any services that
    # rely on it (and ideally be accompanied by PRs to update them accordingly). Known
    # consumers of this API include:
    #
    # * https://github.com/actions/virtual-environments
    #

    if ! VERSION=$(curl --retry 3 --fail --silent -L "https://www.pulumi.com/latest-version"); then
        >&2 say_red "error: could not determine latest version of Pulumi, try passing --version X.Y.Z to"
        >&2 say_red "       install an explicit version"
        exit 1
    fi
fi

OS=""
case $(uname) in
    "Linux") OS="linux";;
    "Darwin") OS="darwin";;
    *)
        print_unsupported_platform
        exit 1
        ;;
esac

ARCH=""
case $(uname -m) in
    "x86_64") ARCH="x64";;
    "arm64") ARCH="arm64";;
    "aarch64") ARCH="arm64";;
    *)
        print_unsupported_platform
        exit 1
        ;;
esac

TARBALL_URL="https://github.com/pulumi/pulumi/releases/download/v\${VERSION}/"
TARBALL_URL_FALLBACK="https://get.pulumi.com/releases/sdk/"
TARBALL_PATH=pulumi-v\${VERSION}-\${OS}-\${ARCH}.tar.gz

PULUMI_INSTALL_ROOT=\${INSTALL_ROOT}
if [ "$PULUMI_INSTALL_ROOT" = "" ]; then
	# Default to ~/.pulumi
	PULUMI_INSTALL_ROOT="\${HOME}/.pulumi"
fi

PULUMI_CLI="\${PULUMI_INSTALL_ROOT}/bin/pulumi"

if [ ! -e "\${PULUMI_CLI}" ]; then
    say_blue "=== Installing Pulumi v\${VERSION} ==="
else
    say_blue "=== Upgrading Pulumi $(\${PULUMI_CLI} version) to v\${VERSION} ==="
fi

TARBALL_DEST=$(mktemp -t pulumi.tar.gz.XXXXXXXXXX)

download_tarball() {
    # Try to download from github first, then fallback to get.pulumi.com
    say_white "+ Downloading \${TARBALL_URL}\${TARBALL_PATH}..."
    # This should opportunistically use the GITHUB_TOKEN to avoid rate limiting
    # ...I think. It's hard to test accurately. But it at least doesn't seem to hurt.
    if ! curl --fail \${SILENT} -L \\
        --header "Authorization: Bearer $GITHUB_TOKEN" \\
        -o "\${TARBALL_DEST}" "\${TARBALL_URL}\${TARBALL_PATH}"; then
        say_white "+ Error encountered, falling back to \${TARBALL_URL_FALLBACK}\${TARBALL_PATH}..."
        if ! curl --retry 2 --fail \${SILENT} -L -o "\${TARBALL_DEST}" "\${TARBALL_URL_FALLBACK}\${TARBALL_PATH}"; then
            return 1
        fi
    fi
}

if download_tarball; then
    say_white "+ Extracting to \${PULUMI_INSTALL_ROOT}/bin"

    # If \`~/.pulumi/bin\` exists, remove previous files with a pulumi prefix
    if [ -e "\${PULUMI_INSTALL_ROOT}/bin/pulumi" ]; then
        rm "\${PULUMI_INSTALL_ROOT}/bin"/pulumi*
    fi

    mkdir -p "\${PULUMI_INSTALL_ROOT}"

    # Yarn's shell installer does a similar dance of extracting to a temp
    # folder and copying to not depend on additional tar flags
    EXTRACT_DIR=$(mktemp -dt pulumi.XXXXXXXXXX)
    tar zxf "\${TARBALL_DEST}" -C "\${EXTRACT_DIR}"

    # Our tarballs used to have a top level bin folder, so support that older
    # format if we detect it. Newer tarballs just have all the binaries in
    # the top level Pulumi folder.
    if [ -d "\${EXTRACT_DIR}/pulumi/bin" ]; then
        mv "\${EXTRACT_DIR}/pulumi/bin" "\${PULUMI_INSTALL_ROOT}/"
    else
        cp -r "\${EXTRACT_DIR}/pulumi/." "\${PULUMI_INSTALL_ROOT}/bin/"
    fi

    rm -f "\${TARBALL_DEST}"
    rm -rf "\${EXTRACT_DIR}"
else
    >&2 say_red "error: failed to download \${TARBALL_URL}"
    >&2 say_red "       check your internet and try again; if the problem persists, file an"
    >&2 say_red "       issue at https://github.com/pulumi/pulumi/issues/new/choose"
    exit 1
fi

# Now that we have installed Pulumi, if it is not already on the path, let's add a line to the
# user's profile to add the folder to the PATH for future sessions.
if [ "\${NO_PATH}" != "true" ]; then
	if ! command -v pulumi >/dev/null; then
		# If we can, we'll add a line to the user's .profile adding \${PULUMI_INSTALL_ROOT}/bin to the PATH
		SHELL_NAME=$(basename "\${SHELL}")
		PROFILE_FILE=""

		case "\${SHELL_NAME}" in
			"bash")
				# Terminal.app on macOS prefers .bash_profile to .bashrc, so we prefer that
				# file when trying to put our export into a profile. On *NIX, .bashrc is
				# preferred as it is sourced for new interactive shells.
				if [ "$(uname)" != "Darwin" ]; then
					if [ -e "\${HOME}/.bashrc" ]; then
						PROFILE_FILE="\${HOME}/.bashrc"
					elif [ -e "\${HOME}/.bash_profile" ]; then
						PROFILE_FILE="\${HOME}/.bash_profile"
					fi
				else
					if [ -e "\${HOME}/.bash_profile" ]; then
						PROFILE_FILE="\${HOME}/.bash_profile"
					elif [ -e "\${HOME}/.bashrc" ]; then
						PROFILE_FILE="\${HOME}/.bashrc"
					fi
				fi
				;;
			"zsh")
				if [ -e "\${ZDOTDIR:-$HOME}/.zshrc" ]; then
					PROFILE_FILE="\${ZDOTDIR:-$HOME}/.zshrc"
				fi
				;;
		esac

		if [ -n "\${PROFILE_FILE}" ]; then
			LINE_TO_ADD="export PATH=\\$PATH:\${PULUMI_INSTALL_ROOT}/bin"
			if ! grep -q "# add Pulumi to the PATH" "\${PROFILE_FILE}"; then
				say_white "+ Adding \${PULUMI_INSTALL_ROOT}/bin to \\$PATH in \${PROFILE_FILE}"
				printf "\\\\n# add Pulumi to the PATH\\\\n%s\\\\n" "\${LINE_TO_ADD}" >> "\${PROFILE_FILE}"
			fi

			EXTRA_INSTALL_STEP="+ Please restart your shell or add \${PULUMI_INSTALL_ROOT}/bin to your \\$PATH"
		else
			EXTRA_INSTALL_STEP="+ Please add \${PULUMI_INSTALL_ROOT}/bin to your \\$PATH"
		fi
	elif [ "$(command -v pulumi)" != "\${PULUMI_INSTALL_ROOT}/bin/pulumi" ]; then
		say_yellow
		say_yellow "warning: Pulumi has been installed to \${PULUMI_INSTALL_ROOT}/bin, but it looks like there's a different copy"
		say_yellow "         on your \\$PATH at $(dirname "$(command -v pulumi)"). You'll need to explicitly invoke the"
		say_yellow "         version you just installed or modify your \\$PATH to prefer this location."
	fi
fi

say_blue
say_blue "=== Pulumi is now installed! 🍹 ==="
if [ "$EXTRA_INSTALL_STEP" != "" ]; then
    say_white "\${EXTRA_INSTALL_STEP}"
fi
say_green "+ Get started with Pulumi: https://www.pulumi.com/docs/quickstart"
`;
