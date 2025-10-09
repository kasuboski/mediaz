---
id: task-1
title: Implement Job storage layer for tracking media jobs
status: In Progress
assignee:
  - '@claude'
created_date: '2025-10-09 18:21'
updated_date: '2025-10-09 18:23'
labels: []
dependencies: []
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
This task aims to implement the storage layer for tracking jobs. The plan is replace the go routines that run the reconcile loop jobs with a new Jobs manager. The Jobs manager will come at a later time.

This ticket aims to lay out the ground work needed for the Job manager create and update jobs.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Database schema for Jobs table
- [ ] #2 CRUD layer created in storage package for Jobs
- [ ] #3 Unit tests added to storage package for jobs
- [ ] #4 Define state machine for Jobs
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
At all times you MUST reference CLAUDE.md before writing code.

We need to begin by defining the schema for the Jobs table to track jobs.

Below is a sample schema that will likely need to be tweaked
`CREATE TABLE IF NOT EXISTS "jobs" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "type" TEXT NOT NULL,
    "created_at" DATETIME DEFAULT CURRENT_TIMESTAMP,
    "updated_at" DATETIME DEFAULT CURRENT_TIMESTAMP,
    "to_state" TEXT NOT NULL,
    "from_state" TEXT,
    "most_recent" BOOLEAN NOT NULL,
    "error" TEXT
);`

Then, we need define the state machine for Jobs and the states they may transition to:
* Pending
* Running
* Error
* Done
*

We can go from..
"" -> "Pending"
"Pending" -> "Running"
"Running" -> "Error"
"Running" -> "Done"
<!-- SECTION:PLAN:END -->
