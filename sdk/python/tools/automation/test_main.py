from __future__ import annotations

from output.main import (
    API,
    PulumiAboutOptions,
    PulumiConfigEnvAddOptions,
    PulumiImportOptions,
    PulumiTemplatePublishOptions,
    PulumiUpOptions,
)


def test_about_command() -> None:
    api = API()
    options: PulumiAboutOptions = {}

    command = api.about(options)
    assert command == "pulumi about"


def test_config_env_add_command() -> None:
    api = API()
    options: PulumiConfigEnvAddOptions = {}

    command = api.config_env_add(options, "my-environment")
    assert command == "pulumi config env add -- my-environment"


def test_template_publish_command() -> None:
    api = API()
    options: PulumiTemplatePublishOptions = {
        "name": "test",
        "version": "1.0.0",
    }

    command = api.template_publish(options, ".")
    assert command == "pulumi template publish --name test --version 1.0.0 -- ."


def test_import_command() -> None:
    api = API()
    options: PulumiImportOptions = {}

    command = api.import_(options, "'aws:iam/user:User'", "name", "id")
    assert command == "pulumi import -- 'aws:iam/user:User' name id"


def test_up_command_with_targets() -> None:
    api = API()
    options: PulumiUpOptions = {"target": ["urnA", "urnB"]}

    command = api.up(options, "https://pulumi.com")
    assert command == "pulumi up --target urnA --target urnB -- https://pulumi.com"

