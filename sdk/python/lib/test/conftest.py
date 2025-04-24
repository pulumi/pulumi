import json

import pytest

from pulumi import Config
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
