from __future__ import annotations

from output.main import API, PulumiImportOptions, PulumiUpOptions


def test_import_command() -> None:
    api = API()
    options: PulumiImportOptions = PulumiImportOptions()  # type: ignore[call-arg]

    command = api.import_(options, "'aws:iam/user:User'", "name", "id")
    assert command == "pulumi import -- 'aws:iam/user:User' name id"


def test_up_command_with_targets() -> None:
    api = API()
    options = PulumiUpOptions() # type: ignore[call-arg]
    options.target = ["urnA", "urnB"]

    command = api.up(options, "https://pulumi.com")
    assert command == "pulumi up --target urnA --target urnB -- https://pulumi.com"

