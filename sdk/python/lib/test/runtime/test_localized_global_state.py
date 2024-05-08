import pytest
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
