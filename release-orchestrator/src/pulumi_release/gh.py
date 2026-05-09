"""
GitHub App authentication + REST/GraphQL helpers.

Lambdas read the App's private key + installation id from Secrets Manager,
then mint short-lived installation tokens via the App-JWT -> exchange flow.
The token is cached in module memory between Lambda invocations (warm
container reuse) and refreshed when within 60s of expiry.
"""

from __future__ import annotations

import base64
import functools
import json
import os
import time
from dataclasses import dataclass
from typing import Any

import boto3
import requests
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import padding


GITHUB_API = "https://api.github.com"
GITHUB_GRAPHQL = "https://api.github.com/graphql"

# Lambda-process-local token cache. Reset on cold start.
_TOKEN_CACHE: dict[str, tuple[str, float]] = {}


@dataclass
class AppConfig:
    app_id: str
    installation_id: str
    private_key_pem: str

    @classmethod
    def from_env(cls) -> "AppConfig":
        secret_arn = os.environ["GITHUB_APP_SECRET_ARN"]
        sm = boto3.client("secretsmanager")
        raw = sm.get_secret_value(SecretId=secret_arn)["SecretString"]
        d = json.loads(raw)
        return cls(
            app_id=str(d["app_id"]),
            installation_id=str(d["installation_id"]),
            private_key_pem=d["private_key"],
        )


def _make_app_jwt(cfg: AppConfig) -> str:
    """RS256-sign a short JWT proving we are the App."""
    now = int(time.time())
    header = {"alg": "RS256", "typ": "JWT"}
    payload = {
        "iat": now - 60,           # account for clock skew
        "exp": now + 540,          # max 10 minutes per GitHub
        "iss": cfg.app_id,
    }

    def b64(d: dict) -> bytes:
        return base64.urlsafe_b64encode(json.dumps(d, separators=(",", ":")).encode()).rstrip(b"=")

    signing_input = b64(header) + b"." + b64(payload)
    key = serialization.load_pem_private_key(cfg.private_key_pem.encode(), password=None)
    signature = key.sign(signing_input, padding.PKCS1v15(), hashes.SHA256())
    sig_b64 = base64.urlsafe_b64encode(signature).rstrip(b"=")
    return (signing_input + b"." + sig_b64).decode()


def _exchange_for_installation_token(cfg: AppConfig) -> tuple[str, float]:
    """Exchange the App JWT for an installation access token. Returns (token, expires_at)."""
    jwt = _make_app_jwt(cfg)
    resp = requests.post(
        f"{GITHUB_API}/app/installations/{cfg.installation_id}/access_tokens",
        headers={
            "Authorization": f"Bearer {jwt}",
            "Accept": "application/vnd.github+json",
        },
        timeout=10,
    )
    resp.raise_for_status()
    body = resp.json()
    # GitHub returns expires_at as ISO8601; convert to epoch.
    from datetime import datetime, timezone
    expires_at = datetime.fromisoformat(body["expires_at"].replace("Z", "+00:00")).timestamp()
    return body["token"], expires_at


def installation_token() -> str:
    """Get a current installation token, minting/exchanging as needed."""
    cfg_arn = os.environ["GITHUB_APP_SECRET_ARN"]
    cached = _TOKEN_CACHE.get(cfg_arn)
    now = time.time()
    if cached and cached[1] - now > 60:
        return cached[0]
    cfg = AppConfig.from_env()
    token, exp = _exchange_for_installation_token(cfg)
    _TOKEN_CACHE[cfg_arn] = (token, exp)
    return token


@functools.lru_cache(maxsize=1)
def _session() -> requests.Session:
    s = requests.Session()
    s.headers.update({
        "Accept": "application/vnd.github+json",
        "User-Agent": "pulumi-release-orchestrator/0.1",
        "X-GitHub-Api-Version": "2022-11-28",
    })
    return s


def _auth_headers() -> dict[str, str]:
    return {"Authorization": f"Bearer {installation_token()}"}


def get(path: str, **kwargs) -> requests.Response:
    """REST GET to api.github.com (path may be absolute or repo-relative)."""
    url = path if path.startswith("http") else f"{GITHUB_API}/{path.lstrip('/')}"
    resp = _session().get(url, headers=_auth_headers(), timeout=30, **kwargs)
    resp.raise_for_status()
    return resp


def post(path: str, json_body: Any | None = None, **kwargs) -> requests.Response:
    url = path if path.startswith("http") else f"{GITHUB_API}/{path.lstrip('/')}"
    resp = _session().post(url, headers=_auth_headers(), json=json_body, timeout=30, **kwargs)
    resp.raise_for_status()
    return resp


def patch(path: str, json_body: Any | None = None, **kwargs) -> requests.Response:
    url = path if path.startswith("http") else f"{GITHUB_API}/{path.lstrip('/')}"
    resp = _session().patch(url, headers=_auth_headers(), json=json_body, timeout=30, **kwargs)
    resp.raise_for_status()
    return resp


def graphql(query: str, variables: dict[str, Any] | None = None) -> dict[str, Any]:
    """POST a GraphQL query."""
    resp = _session().post(
        GITHUB_GRAPHQL,
        headers=_auth_headers(),
        json={"query": query, "variables": variables or {}},
        timeout=30,
    )
    resp.raise_for_status()
    body = resp.json()
    if "errors" in body:
        raise RuntimeError(f"GraphQL errors: {body['errors']}")
    return body["data"]
