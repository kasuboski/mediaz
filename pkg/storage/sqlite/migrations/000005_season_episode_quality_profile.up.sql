-- Issue #101: nullable per-season / per-episode quality profile overrides.
-- NULL means "inherit from the series default profile" — never "any quality".
-- Deleting a referenced profile clears the override back to inheritance
-- instead of blocking the delete (unlike the series-level RESTRICT, an
-- override is optional by definition).

ALTER TABLE "season"
    ADD COLUMN "quality_profile_id" INTEGER
    REFERENCES "quality_profile"("id") ON DELETE SET NULL;

ALTER TABLE "episode"
    ADD COLUMN "quality_profile_id" INTEGER
    REFERENCES "quality_profile"("id") ON DELETE SET NULL;
