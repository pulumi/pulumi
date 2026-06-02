# Copyright 2025, Pulumi Corporation.  All rights reserved.

"""
Pulumi workspace and account logic for python SDK.
This is a partial port of ESC and Pulumi CLI code found in
https://github.com/pulumi/esc/tree/main/cmd/esc/cli/workspace
"""

from dataclasses import dataclass, field
import json
import os
import pathlib
from typing import Optional

from pulumi_esc_sdk.workspace_models import Account, Credentials

def get_pulumi_home_dir() -> str:
    """
    Returns the path of the ".pulumi" folder where Pulumi puts its artifacts.
    """
    # Allow the folder we use to be overridden by an environment variable
    dir_env = os.getenv("PULUMI_HOME")
    if dir_env:
        return dir_env

    # Otherwise, use the current user's home dir + .pulumi
    home_dir = pathlib.Path.home()

    return str(home_dir.joinpath(".pulumi"))

def get_esc_bookkeeping_dir() -> str:
    """
    Returns the path of the ".esc" folder inside Pulumi home dir.
    """
    home_dir = get_pulumi_home_dir()
    return str(pathlib.Path(home_dir).joinpath(".esc"))

def get_path_to_creds_file(dir: str) -> str:
    """
    Returns the path to the esc credentials file on disk.
    """
    return str(pathlib.Path(dir).joinpath("credentials.json"))

def get_esc_current_account_name() -> Optional[str]:
    """
    Returns the current account name from the ESC credentials file.
    """
    creds_file = get_path_to_creds_file(get_esc_bookkeeping_dir())
    try:
        with open(creds_file, "r") as f:
            data = json.loads(f.read())
            return data["name"]
    except FileNotFoundError:
        return None
    except KeyError:
        return None
    except Exception as e:
        print(f"An unexpected error occurred: {e}")
        return None

def get_stored_credentials() -> Credentials:
    """
    Reads and parses credentials from the Pulumi credentials file.
    """
    creds_file = get_path_to_creds_file(get_pulumi_home_dir())
    try:
        with open(creds_file, "r") as f:
            data = json.loads(f.read())
            creds = Credentials.from_json(data)
            return creds
    except FileNotFoundError:
        return None
    except Exception as e:
        print(f"An unexpected error occurred: {e}")
        return None

def get_current_account() -> tuple[Account, str]:
    """
    Gets current account values from credentials file.
    """
    backend_url = get_esc_current_account_name()
    pulumi_credentials = get_stored_credentials()
    if not pulumi_credentials:
        return None, None
    if backend_url is None:
        backend_url = pulumi_credentials.current
    if not backend_url or backend_url not in pulumi_credentials.accounts:
        return None, None
    return pulumi_credentials.accounts[backend_url], backend_url
