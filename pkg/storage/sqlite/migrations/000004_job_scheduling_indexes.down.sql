DROP INDEX IF EXISTS "idx_job_transition_type_state_updated";
DROP INDEX IF EXISTS "idx_job_created_at";
DROP INDEX IF EXISTS "idx_job_type";

PRAGMA foreign_keys=off;

ALTER TABLE "job_transition" RENAME TO "job_transition_new";

CREATE TABLE "job_transition" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "job_id" INTEGER NOT NULL REFERENCES "job"("id"),
    "type" TEXT NOT NULL,
    "to_state" TEXT NOT NULL,
    "from_state" TEXT,
    "most_recent" BOOLEAN NOT NULL,
    "sort_key" INTEGER NOT NULL,
    "error" TEXT,
    "created_at" DATETIME DEFAULT CURRENT_TIMESTAMP,
    "updated_at" DATETIME DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO "job_transition" SELECT * FROM "job_transition_new";

DROP TABLE "job_transition_new";

PRAGMA foreign_keys=on;
