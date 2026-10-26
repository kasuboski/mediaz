# HTTP baseline fixtures

The 61 JSON fixtures were captured against pre-refactor HEAD (`29f62f9`). They
remain the baseline for field spelling/casing, nesting, null versus omitted,
array ordering and JSON key ordering. Do not bulk-regenerate them to make a
refactor pass. Dynamic timestamp/duration/date normalization is in
`server/wire_contract_test.go`.

## Intentional fresh-sqlc deltas

The following fixture edits are targeted edits, not a new capture:

* `download_clients_list`, `download_client_get`, `download_client_create`,
  `download_client_update`, `audit_download_clients_list`, and
  `audit_download_client_get`: remove the `APIKey` property entirely. Stored
  credentials are write-only. Indexer and indexer-source responses already
  omitted credentials and keep their existing shapes.
* `quality_profiles_list`, `quality_profiles_movie`, `quality_profiles_series`,
  and `quality_profile_get`: restore seeded cutoff/upgrade values from the
  canonical initial migration values in `pkg/storage/sqlite/defaults.sql`.
  Profiles 1–6 have cutoffs 2, 8, 13, 16, 23, 27; upgrades are enabled for
  1, 2, 4, 5 and disabled for 3, 6. The old bootstrap snapshot/seed drift
  produced null cutoffs and disabled upgrades in these captures. This is an
  intentional seed bug correction, not a new JSON representation.

* `quality_definitions_list`: remove duplicate seed rows 29–56. The old test
  bootstrap invoked both migration seeding and legacy `Init` seeding; fresh
  `New` seeds once, idempotently. The original 28 definitions are unchanged.
  Accordingly `quality_definition_create` changes ID 57 to 29, and
  `quality_definition_update` changes QualityID 1 to 0: update/delete `/29`
  now genuinely operate on the just-created custom row rather than accidentally
  modifying the first duplicate seed. This preserves the intended custom CRUD
  sequence while fixing the bootstrap bug.
* `jobs_list` and `jobs_list_paginated`: use the intentional stable query
  ordering `(created_at DESC, id DESC)` instead of the old index-dependent
  same-second order. All three jobs are created within one SQLite timestamp
  second in the capture; their expected IDs are now 3, 2, 1 (page one: 3, 2).
* Active activity `stateSince` now comes from the latest transition's actual
  `created_at`, not `time.Unix(sort_key, 0)` (which incorrectly yielded January
  1970). The derived `duration` therefore also changes intentionally. Their
  fixture values already normalize to `<timestamp>` and `<duration>`, so no
  fixture bytes change for this fix. The capture test separately checks that
  freshly seeded active media have recent `stateSince` timestamps, preventing
  normalization from concealing a regression.

* `activity_timeline` and movie/series/episode history fixtures: subsequent
  native transition writes now retain their actual previous state in
  `from_state`; the old writers left it NULL. Their existing optional
  `fromState` property is therefore populated (initial transitions still omit
  it). Timeline same-timestamp ties are now deterministic by entity type
  ascending then transition ID descending. Counts, entities, field names,
  nesting, and chronological direction are unchanged.

## Relational-link cleanup

The baseline now restricts deletion of referenced movie/series quality profiles
and sets optional media metadata/file links to NULL when their referenced row
is deleted. This intentionally prevents dangling links; it does not delete media
bytes. The capture setup already uses real metadata/file rows and valid seeded
quality profiles, so this FK change requires no wire fixture changes. Null links
continue to use the same existing JSON null/omission rules.

No other baseline differences are accepted by this document. In particular,
`sql.Null*` objects must never appear in JSON. Library mutation responses remain
flat historical PascalCase entity fields followed by lowercase `state`; download
IDs/client IDs remain internal and are not added to movie responses.
