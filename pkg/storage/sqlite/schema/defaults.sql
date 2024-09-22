-- Inserting into the indexer table
INSERT INTO "indexer" ("name", "priority", "uri", "api_key") VALUES 
('Indexer1', 10, 'http://example.com/api', 'apikey1'),
('Indexer2', 20, 'http://example.com/api2', 'apikey2'),
('Indexer3', 25, 'http://example.com/api3', NULL);

-- Inserting into the quality_definition table
INSERT INTO "quality_definition" ("quality_id", "name", "preferred_size", "min_size", "max_size", "media_type") VALUES 
(1, 'HDTV-720p', 1999, 1700, 2000, 'movie'),
(2, 'HDTV-1080p', 1999, 3300, 4000, 'movie'),
(3, '4K UHD', 1999, 15000, 20000, 'movie');

-- Inserting into the quality_profile table
INSERT INTO "quality_profile" ("name", "cutoff", "upgrade_allowed") VALUES 
('Standard Quality', 1, TRUE),
('HD Quality', 2, TRUE),
('Ultra HD Quality', 3, FALSE);

-- Inserting into the quality_item table
INSERT INTO "quality_item" ("quality_id", "name", "allowed", "parent_id") VALUES 
(1, 'SD - 480p', TRUE, NULL),
(1, 'HD - 720p', TRUE, NULL),
(2, 'HD - 1080p', TRUE, NULL),
(3, '4K - UHD', TRUE, NULL),
(1, 'HD - 720p - Remastered', TRUE, 1); -- Example of a child item

-- Inserting into the sub_quality_item table
INSERT INTO "sub_quality_item" ("quality_id", "name", "allowed", "parent_id") VALUES 
(1, 'Sub-item for HD - 720p', TRUE, 2),
(2, 'Extended Cut - HD - 1080p', TRUE, 3); 

-- Inserting into the profile_quality_item table
INSERT INTO "profile_quality_item" ("profile_id", "quality_item_id") VALUES 
(1, 1),  -- Standard Quality includes SD - 480p
(1, 2),  -- Standard Quality includes HD - 720p
(2, 3),  -- HD Quality includes HD - 1080p
(3, 4);  -- Ultra HD Quality includes 4K UHD
