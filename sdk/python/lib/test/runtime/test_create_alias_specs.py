import pytest
from pulumi.runtime.resource import create_alias_spec
from pulumi.resource import Alias


@pytest.mark.asyncio
async def test_create_alias_spec_empty():
    alias = Alias()
    alias_spec = await create_alias_spec(alias)
    assert alias_spec is not None
    assert alias_spec.name == ""
    assert alias_spec.type == ""
    assert alias_spec.project == ""
    assert alias_spec.parentUrn == ""
    assert alias_spec.noParent is False


@pytest.mark.asyncio
async def test_create_alias_spec_basic():
    alias = Alias(name="foo")
    alias_spec = await create_alias_spec(alias)
    assert alias_spec is not None
    assert alias_spec.name == "foo"


@pytest.mark.asyncio
async def test_create_alias_spec_with_type():
    alias = Alias(type_="foo")
    alias_spec = await create_alias_spec(alias)
    assert alias_spec is not None
    assert alias_spec.type == "foo"


@pytest.mark.asyncio
async def test_create_alias_spec_with_parent():
    alias = Alias(parent="pulumi:pulumi:Stack")
    alias_spec = await create_alias_spec(alias)
    assert alias_spec is not None
    assert alias_spec.noParent is False
    assert alias_spec.parentUrn == "pulumi:pulumi:Stack"
