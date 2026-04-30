import asyncio

import pytest
from pulumi.runtime import settings
from pulumi.runtime.settings import Settings


@pytest.mark.asyncio
async def test_settings():
    default_settings_instance = Settings("project", "stack")

    async def set_and_retrieve_settings_value(stack_name: str):
        default_settings_instance.stack = stack_name
        assert default_settings_instance.stack == stack_name
        return default_settings_instance.stack

    foo_name = await set_and_retrieve_settings_value("foo")
    bar_name = await set_and_retrieve_settings_value("bar")
    assert foo_name != bar_name != "project"
    assert default_settings_instance.project == "project"


def test_package_refs_reset_with_settings():
    old_settings = settings.SETTINGS
    package_key = ("parameterized", "1.0.0")

    try:
        settings.configure(Settings("project", "stack"))
        settings.SETTINGS.feature_support["parameterization"] = True
        asyncio.run(settings.set_package_ref(package_key, "ref-1"))
        assert asyncio.run(settings.get_package_ref(package_key)) == "ref-1"

        settings.configure(Settings("project", "stack"))
        settings.SETTINGS.feature_support["parameterization"] = True
        assert asyncio.run(settings.get_package_ref(package_key)) is ...
    finally:
        settings.configure(old_settings)
