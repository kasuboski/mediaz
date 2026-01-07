PRAGMA foreign_keys=off;

ALTER TABLE "job_transition" RENAME TO "job_transition_old";

CREATE TABLE "job_transition" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "job_id" INTEGER NOT NULL REFERENCES "job"("id") ON DELETE CASCADE,
    "type" TEXT NOT NULL,
    "to_state" TEXT NOT NULL,
    "from_state" TEXT,
    "most_recent" BOOLEAN NOT NULL,
    "sort_key" INTEGER NOT NULL,
    "error" TEXT,
    "created_at" DATETIME DEFAULT CURRENT_TIMESTAMP,
    "updated_at" DATETIME DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO "job_transition" SELECT * FROM "job_transition_old";

DROP TABLE "job_transition_old";

PRAGMA foreign_keys=on;

CREATE INDEX IF NOT EXISTS "idx_job_transition_type_state_updated"
ON "job_transition"("type", "to_state", "updated_at" DESC)
WHERE "most_recent" = 1;

CREATE INDEX IF NOT EXISTS "idx_job_created_at"
ON "job"("created_at" DESC);

CREATE INDEX IF NOT EXISTS "idx_job_type"
ON "job"("type");
