# `GET` /api/orgs/{orgName}/tokens

_ListOrgTokens_

**Operation:** `ListOrgTokens` · **Tag:** AccessTokens

## Description

Retrieves all access tokens created for an organization. Organization tokens provide CI/CD automation access scoped to the organization rather than tied to individual user accounts. The response includes token metadata such as name, description, creation date, last used date, and expiration status. The actual token values are never returned after initial creation. An optional filter parameter can include expired tokens in the results.

## Parameters

- `orgName` `string` _(in: path)_ **required** — The organization name
- `filter` `string` _(in: query)_ _optional_ — Filter tokens by status (e.g., include expired tokens)

## Responses

### `200 OK`

- `tokens` `array[object]` **required** — The list of access tokens
  - `id` `string` **required** — Unique identifier for this access token.
  - `name` `string` **required** — Human-readable name assigned to this access token.
  - `description` `string` **required** — User-provided description of the token's purpose.
  - `created` `string` **required** — Timestamp when the token was created, in ISO 8601 format.
  - `lastUsed` `integer (int64)` **required** — Unix epoch timestamp (seconds) when the token was last used. Zero if never used.
  - `expires` `integer (int64)` **required** — Unix epoch timestamp (seconds) when the token expires. Zero if it never expires.
  - `admin` `boolean` **required** — Whether this token has Pulumi Cloud admin privileges.
  - `createdBy` `string` **required** — User.GitHubLogin of the user that created the access token
  - `role` `object` — A role that can be associated with an access token to scope its permissions.
    - `id` `string` **required** — Unique identifier for this role.
    - `name` `string` **required** — Display name of the role.
    - `defaultIdentifier` `string enum` **required** _enum:_ `member`, `admin`, `billing-manager`, `stack-read`, `stack-write`, `stack-admin`, … — The default identity to assume when using a token with this role.
