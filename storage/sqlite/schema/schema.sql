CREATE TABLE sqlite_sequence(name,seq);
CREATE TABLE IF NOT EXISTS "Indexers" ("Id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT, "Name" TEXT NOT NULL, "Implementation" TEXT NOT NULL, "Settings" TEXT, "ConfigContract" TEXT, "EnableRss" INTEGER, "EnableAutomaticSearch" INTEGER, "EnableInteractiveSearch" INTEGER NOT NULL, "Priority" INTEGER NOT NULL DEFAULT 25, "Tags" TEXT, "DownloadClientId" INTEGER NOT NULL DEFAULT 0);
CREATE UNIQUE INDEX "IX_Indexers_Name" ON "Indexers" ("Name" ASC);
