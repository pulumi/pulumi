# `POST` /api/orgs/{orgName}/tokens

_CreateOrgToken_

**Operation:** `CreateOrgToken` · **Tag:** AccessTokens

## Description

Generates a new access token scoped to the organization for use in CI/CD pipelines and automated workflows. Organization tokens belong to the organization rather than individual users, ensuring that access is not disrupted when team members leave.

The `name` field must be unique across the organization (including deleted tokens) and cannot exceed 40 characters. The `expires` field accepts a unix epoch timestamp up to two years from the present, or `0` for no expiry (default).

**Important:** The token value in the response is only returned once at creation time and cannot be retrieved later. Audit logs for actions performed with organization tokens are attributed to the organization rather than an individual user.

## Parameters

- `orgName` `string` _(in: path)_ **required** — The organization name
- `reason` `string` _(in: query)_ _optional_ — Audit log reason for creating this token

## Request body (`application/json`)

- `description` `string` **required** — The description
- `name` `string` **required** — The name
- `admin` `boolean` **required** — Whether the entity has admin privileges
- `expires` `integer (int64)` **required** — The expiration time
- `roleID` `string` — The role identifier

## Responses

### `200 OK`

- `id` `string` **required** — The unique identifier
- `tokenValue` `string` **required** — The token value
