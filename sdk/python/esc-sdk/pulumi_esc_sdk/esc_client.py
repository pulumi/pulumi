
# Copyright 2024, Pulumi Corporation.  All rights reserved.

from pulumi_esc_sdk.exceptions import ApiException
import pulumi_esc_sdk.models as models
import pulumi_esc_sdk.api as api
import pulumi_esc_sdk.configuration as configuration
import pulumi_esc_sdk.api_client as api_client
from pydantic import StrictBytes, StrictInt
from typing import Mapping, Any, List
import inspect
import yaml
import os
from urllib.parse import urlparse, urlunparse
import pulumi_esc_sdk.workspace as workspace


class EscClient:
    """EscClient is a client for the ESC API.
    It wraps the raw API client and provides a more convenient interface.

    :param configuration: API client configuration.
    """
    esc_api: api.EscApi = None

    def __init__(self, configuration: configuration.Configuration) -> None:
        """Constructor
        """
        configuration.host = append_esc_to_url(configuration.host)
        self.esc_api = api.EscApi(api_client.ApiClient(configuration))

    def list_environments(self, org_name: str,
                          continuation_token: str = None) -> models.OrgEnvironments:
        """List all environments in an organization.

        :param org_name: The name of the organization.
        :param continuation_token: The continuation token to use for pagination.
        :return: The list of environments.
        """
        return self.esc_api.list_environments(org_name, continuation_token)

    def get_environment(self, org_name: str, project_name: str,
                        env_name: str) -> tuple[models.EnvironmentDefinition, StrictBytes]:
        """Get an environment by name.

        :param org_name: The name of the organization.
        :param project_name: The name of the project.
        :param env_name: The name of the environment.
        :return: The environment definition and the raw data.
        """
        response = self.esc_api.get_environment_with_http_info(org_name, project_name, env_name)
        return response.data, response.raw_data

    def get_environment_at_version(
            self, org_name: str, project_name: str, env_name: str,
            version: str) -> tuple[models.EnvironmentDefinition, StrictBytes]:
        """Get an environment by name and version.

        :param org_name: The name of the organization.
        :param project_name: The name of the project.
        :param env_name: The name of the environment.
        :param version: The version of the environment.
        :return: The environment definition and the raw data.
        """
        response = self.esc_api.get_environment_at_version_with_http_info(
            org_name, project_name, env_name, version)
        return response.data, response.raw_data

    def open_environment(self, org_name: str, project_name: str,
                         env_name: str) -> models.OpenEnvironment:
        """Open an environment for reading.

        :param org_name: The name of the organization.
        :param project_name: The name of the project.
        :param env_name: The name of the environment.
        :return: The opened environment.
        """
        return self.esc_api.open_environment(org_name, project_name, env_name)

    def open_environment_at_version(
            self, org_name: str, project_name: str, env_name: str,
            version: str) -> models.OpenEnvironment:
        """Open an environment for reading at a specific version.

        :param org_name: The name of the organization.
        :param project_name: The name of the project.
        :param env_name: The name of the environment.
        :param version: The version of the environment.
        :return: The opened environment."""
        return self.esc_api.open_environment_at_version(org_name, project_name, env_name, version)

    def read_open_environment(
            self, org_name: str, project_name: str, env_name: str,
            open_session_id: str) -> tuple[models.Environment, Mapping[str, Any], str]:
        """Read an open environment and resolves config and data.

        :param org_name: The name of the organization.
        :param project_name: The name of the project.
        :param env_name: The name of the environment.
        :param open_session_id: The open session identifier.
        :return: The environment, the values, and the raw data."""
        response = self.esc_api.read_open_environment_with_http_info(
            org_name, project_name, env_name, open_session_id)
        values = convertEnvPropertiesToValues(response.data.properties)
        return response.data, values, response.raw_data.decode('utf-8')

    def open_and_read_environment(
            self, org_name: str, project_name: str,
            env_name: str) -> tuple[models.Environment, Mapping[str, Any], str]:
        """Open and read an environment and resolves config and data.

        :param org_name: The name of the organization.
        :param project_name: The name of the project.
        :param env_name: The name of the environment.
        :return: The environment, the values, and the raw data.
        """
        openEnv = self.open_environment(org_name, project_name, env_name)
        return self.read_open_environment(org_name, project_name, env_name, openEnv.id)

    def open_and_read_environment_at_version(
            self, org_name: str, project_name: str, env_name: str,
            version: str) -> tuple[models.Environment, Mapping[str, Any], str]:
        """Open and read an environment at a specific version and resolves config and data.

        :param org_name: The name of the organization.
        :param project_name: The name of the project.
        :param env_name: The name of the environment.
        :param version: The version of the environment.
        :return: The environment, the values, and the raw data.
        """
        openEnv = self.open_environment_at_version(org_name, project_name, env_name, version)
        return self.read_open_environment(org_name, project_name, env_name, openEnv.id)

    def read_open_environment_property(
            self, org_name: str, project_name: str, env_name: str,
            open_session_id: str, property_name: str) -> tuple[models.Value, Any]:
        """Read a property from an open environment and resolves the value.

        :param org_name: The name of the organization.
        :param project_name: The name of the project.
        :param env_name: The name of the environment.
        :param open_session_id: The open session identifier.
        :param property_name: The property name.
        :return: The property value and the resolved value.
        """
        v = self.esc_api.read_open_environment_property(
            org_name, project_name, env_name, open_session_id, property_name)
        return v, convertPropertyToValue(v.value)

    def create_environment(self, org_name: str, project_name: str,
                           env_name: str) -> models.Environment:
        """Create an environment.

        :param org_name: The name of the organization.
        :param project_name: The name of the project.
        :param env_name: The name of the environment.
        :return: The created environment."""
        createEnv = models.CreateEnvironment(project=project_name, name=env_name)
        return self.esc_api.create_environment(org_name, createEnv)

    def clone_environment(
            self, org_name: str, src_project_name: str, src_env_name: str,
            dest_project_name: str, dest_env_name: str,
            clone_options: dict = {}) -> models.Environment:
        """Clone an environment.

        :param org_name: The name of the organization.
        :param src_project_name: The name of the source project.
        :param src_env_name: The name of the source environment.
        :param dest_project_name: The name of the destination project.
        :param dest_env_name: The name of the destination environment.
        :param clone_options: A dictionary containing clone options.
        :key bool preserve_access: Whether to preserve team access.
        :key bool preserve_environment_tags: Whether to preserve tags.
        :key bool preserve_history: Whether to preserve history.
        :key bool preserve_revision_tags: Whether to preserve version tags.
        :return: The created environment."""

        cloneEnv = models.CloneEnvironment(
            project=dest_project_name, name=dest_env_name, **clone_options)
        return self.esc_api.clone_environment(org_name, src_project_name, src_env_name, cloneEnv)

    def update_environment_yaml(
            self, org_name: str, project_name: str, env_name: str,
            yaml_body: str) -> models.EnvironmentDiagnostics:
        """Update an environment using the YAML body.

        :param org_name: The name of the organization.
        :param project_name: The name of the project.
        :param env_name: The name of the environment.
        :param yaml_body: The YAML text.
        :return: The environment diagnostics."""
        return self.esc_api.update_environment_yaml(org_name, project_name, env_name, yaml_body)

    def update_environment(
            self, org_name: str, project_name: str, env_name: str,
            env: models.EnvironmentDefinition) -> models.Environment:
        """Update an environment using the environment definition.

        :param org_name: The name of the organization.
        :param project_name: The name of the project.
        :param env_name: The name of the environment.
        :param env: The environment definition.
        :return: The updated environment."""
        envData = env.to_dict()
        yaml_body = yaml.dump(envData)
        return self.update_environment_yaml(org_name, project_name, env_name, yaml_body)

    def delete_environment(self, org_name: str, project_name: str, env_name: str) -> None:
        """Delete an environment.

        :param org_name: The name of the organization.
        :param project_name: The name of the project.
        :param env_name: The name of the environment.
        """
        self.esc_api.delete_environment(org_name, project_name, env_name)

    def check_environment_yaml(self, org_name: str, yaml_body: str) -> models.CheckEnvironment:
        """Check an environment using the YAML body.

        :param org_name: The name of the organization.
        :param yaml_body: The YAML text.
        :return: The check environment result with diagnostics."""
        try:
            response = self.esc_api.check_environment_yaml_with_http_info(org_name, yaml_body)
            return response.data
        except ApiException as e:
            return e.data

    def check_environment(self, org_name: str,
                          env: models.EnvironmentDefinition) -> models.CheckEnvironment:
        """Check an environment using the environment definition.

        :param org_name: The name of the organization.
        :param env: The environment definition.
        :return: The check environment result with diagnostics."""
        yaml_body = yaml.safe_dump(env.to_dict())
        return self.check_environment_yaml(org_name, yaml_body)

    def decrypt_environment(
            self, org_name: str, project_name: str,
            env_name: str) -> tuple[models.EnvironmentDefinition, str]:
        """Decrypt an environment.

        :param org_name: The name of the organization.
        :param project_name: The name of the project.
        :param env_name: The name of the environment.
        :return: The decrypted environment and the raw data."""
        response = self.esc_api.decrypt_environment_with_http_info(org_name, project_name, env_name)
        return response.data, response.raw_data.decode('utf-8')

    def list_environment_revisions(
            self,
            org_name: str,
            project_name: str,
            env_name: str,
            before: StrictInt | None = None,
            count: StrictInt | None = None
            ) -> List[models.EnvironmentRevision]:
        """List environment revisions, from newest to oldest.

        :param org_name: The name of the organization.
        :param project_name: The name of the project.
        :param env_name: The name of the environment.
        :param before: The revision before which to list.
        :param count: The number of revisions to list."""
        return self.esc_api.list_environment_revisions(
            org_name, project_name, env_name, before, count)

    def list_environment_revision_tags(
            self,
            org_name: str,
            project_name: str,
            env_name: str,
            after: str | None = None,
            count: StrictInt | None = None
            ) -> models.EnvironmentRevisionTags:
        """List environment revision tags.

        :param org_name: The name of the organization.
        :param project_name: The name of the project.
        :param env_name: The name of the environment.
        :param after: The tag after which to list.
        :param count: The number of tags to list."""
        return self.esc_api.list_environment_revision_tags(
            org_name, project_name, env_name, after, count)

    def create_environment_revision_tag(
            self, org_name: str, project_name: str, env_name: str,
            tag_name: str, revision: StrictInt) -> None:
        """Create an environment revision tag.

        :param org_name: The name of the organization.
        :param project_name: The name of the project.
        :param env_name: The name of the environment.
        :param tag_name: The name of the tag.
        :param revision: The revision to tag.
        """
        create_tag = models.CreateEnvironmentRevisionTag(name=tag_name, revision=revision)
        return self.esc_api.create_environment_revision_tag(
            org_name, project_name, env_name, create_tag)

    def update_environment_revision_tag(
            self, org_name: str, project_name: str, env_name: str,
            tag_name: str, revision: StrictInt) -> None:
        """Update an environment revision tag.

        :param org_name: The name of the organization.
        :param project_name: The name of the project.
        :param env_name: The name of the environment.
        :param tag_name: The name of the tag.
        :param revision: The revision to tag.
        """
        update_tag = models.UpdateEnvironmentRevisionTag(revision=revision)
        return self.esc_api.update_environment_revision_tag(
            org_name, project_name, env_name, tag_name, update_tag)

    def get_environment_revision_tag(
            self, org_name: str, project_name: str, env_name: str,
            tag_name: str) -> models.EnvironmentRevisionTag:
        """Get an environment revision tag.

        :param org_name: The name of the organization.
        :param project_name: The name of the project.
        :param env_name: The name of the environment.
        :param tag_name: The name of the tag.
        :return: The environment revision tag."""
        return self.esc_api.get_environment_revision_tag(org_name, project_name, env_name, tag_name)

    def delete_environment_revision_tag(
            self, org_name: str, project_name: str, env_name: str, tag_name: str) -> None:
        """Delete an environment revision tag.

        :param org_name: The name of the organization.
        :param project_name: The name of the project.
        :param env_name: The name of the environment.
        :param tag_name: The name of the tag.
        """
        self.esc_api.delete_environment_revision_tag(org_name, project_name, env_name, tag_name)

    def list_environment_tags(
            self,
            org_name: str,
            project_name: str,
            env_name: str,
            after: str | None = None,
            count: StrictInt | None = None) -> models.ListEnvironmentTags:
        """List environment tags.

        :param org_name: The name of the organization.
        :param project_name: The name of the project.
        :param env_name: The name of the environment.
        :param after: The tag after which to list.
        :param count: The number of tags to list.
        :return: The environment tags."""
        return self.esc_api.list_environment_tags(org_name, project_name, env_name, after, count)

    def get_environment_tag(
            self, org_name: str, project_name: str, env_name: str,
            tag_name: str) -> models.EnvironmentTag:
        """Get an environment tag.

        :param org_name: The name of the organization.
        :param project_name: The name of the project.
        :param env_name: The name of the environment.
        :param tag_name: The name of the tag.
        :return: The environment tag."""
        return self.esc_api.get_environment_tag(org_name, project_name, env_name, tag_name)

    def create_environment_tag(
            self, org_name: str, project_name: str, env_name: str,
            tag_name: str, tag_value: str) -> models.EnvironmentTag:
        """Create an environment tag.

        :param org_name: The name of the organization.
        :param project_name: The name of the project.
        :param env_name: The name of the environment.
        :param tag_name: The name of the tag.
        :param tag_value: The value of the tag.
        :return: The created environment tag."""
        create_tag = models.CreateEnvironmentTag(name=tag_name, value=tag_value)
        return self.esc_api.create_environment_tag(org_name, project_name, env_name, create_tag)

    def update_environment_tag(
            self, org_name: str, project_name: str, env_name: str, tag_name: str,
            current_tag_value: str, new_tag_name: str, new_tag_value: str) -> models.EnvironmentTag:
        """Update an environment tag.

        :param org_name: The name of the organization.
        :param project_name: The name of the project.
        :param env_name: The name of the environment.
        :param tag_name: The name of the tag.
        :param current_tag_value: The current value of the tag.
        :param new_tag_name: The new name of the tag.
        :param new_tag_value: The new value of the tag.
        :return: The updated environment tag."""
        update_tag = models.UpdateEnvironmentTag(
            currentTag=models.UpdateEnvironmentTagCurrentTag(value=current_tag_value),
            newTag=models.UpdateEnvironmentTagNewTag(name=new_tag_name, value=new_tag_value)
        )
        return self.esc_api.update_environment_tag(
            org_name, project_name, env_name, tag_name, update_tag)

    def delete_environment_tag(
            self, org_name: str, project_name: str, env_name: str, tag_name: str) -> None:
        """Delete an environment tag.

        :param org_name: The name of the organization.
        :param project_name: The name of the project.
        :param env_name: The name of the environment.
        :param tag_name: The name of the tag.
        """
        self.esc_api.delete_environment_tag(org_name, project_name, env_name, tag_name)


def convertEnvPropertiesToValues(env: Mapping[str, models.Value]) -> Any:
    if env is None:
        return env

    values = {}
    for key in env:
        value = env[key]

        values[key] = convertPropertyToValue(value.value)

    return values


def convertPropertyToValue(property: Any) -> Any:
    if property is None:
        return property

    value = property
    if isinstance(property, dict) and "value" in property:
        value = convertPropertyToValue(property["value"])
        return value

    if value is None:
        return value

    if type(value) is list:
        result = []
        for item in value:
            result.append(convertPropertyToValue(item))
        return result

    if isObject(value):
        result = {}
        for key in value:
            result[key] = convertPropertyToValue(value[key])
        return result

    return value


def isObject(obj):
    return inspect.isclass(obj) or isinstance(obj, dict)


def append_esc_to_url(custom_backend_url_str):
    if custom_backend_url_str is None:
        return None
    try:
        custom_backend_url = urlparse(custom_backend_url_str)
        appended_url = urlunparse((
            str(custom_backend_url.scheme),
            str(custom_backend_url.netloc),
            "/api/esc",
            None,  # path
            None,  # query
            None,  # fragment
        ))
        return appended_url
    except Exception as e:
        print(f"Error parsing URL: {e}")
        return None


def default_config(host=None,
                   access_token=None,
                   server_index=None, server_variables=None,
                   server_operation_index=None, server_operation_variables=None,
                   ssl_ca_cert=None,
                   ) -> configuration.Configuration:
    """Creates default configuration for EscClient.
    """
    if not access_token:
        access_token = os.getenv("PULUMI_ACCESS_TOKEN")
    if not host:
        host = os.getenv("PULUMI_BACKEND_URL")

    if not access_token or not host:
        account, backend_url = workspace.get_current_account()
        if not access_token:
            access_token = account.accessToken if account else None
        if not host:
            host = backend_url

    return configuration.Configuration(
        host=host,
        access_token=access_token,
        server_index=server_index,
        server_variables=server_variables,
        server_operation_index=server_operation_index,
        server_operation_variables=server_operation_variables,
        ssl_ca_cert=ssl_ca_cert)


def default_client(host=None,
                   access_token=None,
                   server_index=None, server_variables=None,
                   server_operation_index=None, server_operation_variables=None,
                   ssl_ca_cert=None,
                   ) -> EscClient:
    """Creates default EscClient.
    """
    return EscClient(default_config(
        host=host,
        access_token=access_token,
        server_index=server_index,
        server_variables=server_variables,
        server_operation_index=server_operation_index,
        server_operation_variables=server_operation_variables,
        ssl_ca_cert=ssl_ca_cert))
