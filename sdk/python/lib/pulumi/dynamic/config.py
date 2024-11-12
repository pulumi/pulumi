from typing import Any, Dict

_config_separator = ":"


class Config:
    """
    Config is a bag of configuration values that can be passed to a provider's configure method.

    Use the Config.get and Config.require methods to retrieve a configuration value by key.
    """

    _props: Dict[str, Any]
    _project_name: str

    def __init__(self, raw_config: Dict[str, Any], project_name: str) -> None:
        self._props = raw_config
        self._project_name = project_name

    def get(self, key: str) -> Any:
        """
        get retrieves a configuration value by key. Returns None if the key is not present.
        If no namespace is provided in the key, the project name will be used as the namespace.
        """
        if _config_separator not in key:
            key = self._project_name + _config_separator + key
        return self._props.get(key)

    def require(self, key: str) -> Any:
        """
        require retrieves a configuration value by key. Returns an error if the key is not present.
        If no namespace is provided in the key, the project name will be used as the namespace.
        """
        val = self.get(key)
        if val is None:
            raise ValueError(f"missing required configuration key: {key}")
        return val
