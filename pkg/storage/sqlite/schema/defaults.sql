-- Inserting into the quality_definition table
INSERT INTO quality_definition (quality_id, name, preferred_size, min_size, max_size, media_type) 
VALUES 
(1, 'HDTV-720p', 1999, 17.1, 2000, 'movie'),
(2, 'WEBDL-720p', 1999, 12.5, 2000, 'movie'),
(3, 'WEBRip-720p', 1999, 12.5, 2000, 'movie'),
(4, 'Bluray-720p', 1999, 25.7, 2000, 'movie'),
(5, 'HDTV-1080p', 1999, 33.8, 2000, 'movie'),
(6, 'WEBDL-1080p', 1999, 12.5, 2000, 'movie'),
(7, 'WEBRip-1080p', 1999, 12.5, 2000, 'movie'),
(8, 'Bluray-1080p', 1999, 50.8, 2000, 'movie'),
(9, 'Remux-1080p', 1999, 102, 2000, 'movie'),
(10, 'HDTV-2160p', 1999, 85, 2000, 'movie'),
(11, 'WEBDL-2160p', 1999, 34.5, 2000, 'movie'),
(12, 'WEBRip-2160p', 1999, 34.5, 2000, 'movie'),
(13, 'Bluray-2160p', 1999, 102, 2000, 'movie'),
(14, 'Remux-2160p', 1999, 187.4, 2000, 'movie');

INSERT INTO "quality_profile" ("name", "cutoff", "upgrade_allowed") 
VALUES 
('Standard Definition', 2, TRUE),
('High Definition', 8, TRUE),
('Ultra High Definition', 13, FALSE);

INSERT INTO "quality_item" ("quality_id", "name", "allowed", "parent_id") 
VALUES 
(1, 'SDTV', TRUE, NULL),
(2, 'DVD', TRUE, NULL),
(3, 'HDTV-720p', TRUE, NULL),
(4, 'WEBDL-720p', TRUE, NULL),
(5, 'Bluray-720p', TRUE, NULL),
(6, 'HDTV-1080p', TRUE, NULL),
(7, 'WEBDL-1080p', TRUE, NULL),
(8, 'Bluray-1080p', TRUE, NULL),
(9, 'Remux-1080p', TRUE, NULL),
(10, 'HDTV-2160p', TRUE, NULL),
(11, 'WEBDL-2160p', TRUE, NULL),
(12, 'Bluray-2160p', TRUE, NULL),
(13, 'Remux-2160p', TRUE, NULL);


INSERT INTO "sub_quality_item" ("quality_id", "name", "allowed", "parent_id") 
VALUES 
(3, 'HDTV-720p Extended Cut', TRUE, 3),
(6, 'HDTV-1080p Extended Cut', TRUE, 6);

-- Inserting into the profile_quality_item table
INSERT INTO "profile_quality_item" ("profile_id", "quality_item_id") 
VALUES 
(1, 1),  -- Standard Definition includes SDTV
(1, 2),  -- Standard Definition includes DVD
(2, 3),  -- High Definition includes HDTV-720p
(2, 4),  -- High Definition includes WEBDL-720p
(2, 5),  -- High Definition includes Bluray-720p
(2, 6),  -- High Definition includes HDTV-1080p
(2, 7),  -- High Definition includes WEBDL-1080p
(2, 8),  -- High Definition includes Bluray-1080p
(3, 9),  -- Ultra High Definition includes Remux-1080p
(3, 10), -- Ultra High Definition includes HDTV-2160p
(3, 11), -- Ultra High Definition includes WEBDL-2160p
(3, 12), -- Ultra High Definition includes Bluray-2160p
(3, 13); -- Ultra High Definition includes Remux-2160p
