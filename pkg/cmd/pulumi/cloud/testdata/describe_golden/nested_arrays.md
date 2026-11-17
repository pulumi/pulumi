# `GET` /api/test/nested-arrays/{id}

_GetNestedArrays_

**Operation:** `GetNestedArrays` · **Tag:** TestNested

## Description

Regression coverage: response has both an array of allOf-wrapped objects and an array of string-enum primitives. Exercises markdown renderer paths that were silently dropping nested model fields and inline enum values.

## Parameters

- `id` `string` _(in: path)_ **required** — Synthetic id.

## Responses

### `200 OK` — Nested-array fixture response.

- `owners` `array[object]` **required** — Owners list — items use allOf to merge in the OwnerSummary shape.
  - `name` `string` **required** — Owner display name.
  - `role` `string enum` _enum:_ `admin`, `member`, `viewer` — Owner role.
- `statuses` `array[string enum]` **required** — Allowed status values for the thing.
  - `string enum` _enum:_ `pending`, `active`, `archived`
