# TV acquisition admission and recovery

TV manual requests and scheduled acquisition share final per-series admission.
Selection/profile assignment saves share that lock. Searches run outside it;
current eligibility and profile assignments are checked again before submission.
A pack claims its entire known season episode set atomically before external I/O.
Existing active season jobs conservatively cover the whole season.

Migration 000006 adds `tv_episode_admission(episode_id, created_at)`. Rows are
persistent safety blocks, not jobs and not evidence that a downloader accepted
anything. They are removed after all episode download linkage is stored, or when
a client explicitly proves rejection (`download.SubmissionRejectedError`). Local
client construction failures occur before claiming. Network errors, malformed
responses, a failed post-add status lookup, and process crashes retain claims.
There is intentionally no automatic expiration/retry: time passing does not
prove that an external job was not created. Requests report an admission block
or uncertain outcome; scheduled failures are logged. The UI's 202 remains only
"request processed", not proof of submission.

## Manual recovery (no recovery HTTP API)

1. Stop Mediaz so no requests, scheduler runs, or selection saves are active.
   Back up the SQLite database. Do not run two Mediaz writers against one DB;
   the policy/admission mutex is process-local (the episode claims are durable).
2. Inspect claims with a trusted SQLite client:
   ```sql
   SELECT a.episode_id, a.created_at, e.season_id, e.episode_number
   FROM tv_episode_admission a JOIN episode e ON e.id=a.episode_id;
   ```
3. Verify the downloader's queue AND history for the release and all covered
   episodes. If acceptance cannot be ruled out, retain the claims. Do not clear
   a claim merely because Mediaz has no download ID. Pack claims must be treated
   as a group; keep all until the full pack outcome is understood.
4. If the downloader definitely has no accepted job (or its job has been
   deliberately resolved by the operator), remove only the verified claims in
   a transaction, using explicit local episode IDs:
   ```sql
   BEGIN IMMEDIATE;
   DELETE FROM tv_episode_admission WHERE episode_id IN (/* verified IDs */);
   COMMIT;
   ```
   If episode transitions already reference an active job, leave those intact;
   normal progress reconciliation must finish it. A retry is only appropriate
   for episodes still missing after recovery.
5. Restart Mediaz and request again only if still desired. Selection/quality
   changes never cancel or replace an existing job.

This is conservative duplicate prevention, not arbitrary-downloader exactly-once
execution. An external Add and SQLite linkage cannot be committed atomically.
A crash after claiming but before sending can require unnecessary manual
recovery; this is preferable to blind duplicate submission. Claims are not a
release browser, cancellation mechanism, or generic workflow engine.
