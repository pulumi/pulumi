import json

import pytest

from pulumi.config import Config
from pulumi.runtime.config import set_all_config


@pytest.fixture
def config_settings():
    stack_name = "test-config"
    return {
        f"{stack_name}:string": "bar",
        f"{stack_name}:int": "1",
        f"{stack_name}:bool": "False",
        f"{stack_name}:object": json.dumps({"banana": "sundae"}),
        f"{stack_name}:float": "3.14159",
    }


@pytest.fixture
def mock_config(config_settings):
    set_all_config(config_settings)
    return Config("test-config")


def pytest_collection_modifyitems(items):
    for i, item in enumerate(items):
        # We need to run `test_automation_api_in_forked_worker` first before any other test sets up grpc.aio.
        if item.name == "test_automation_api_in_forked_worker":
            items.insert(0, items.pop(i))
            break
