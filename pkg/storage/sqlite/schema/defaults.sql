-- Quality Definitions for Movies
INSERT INTO
    quality_definition (
        quality_id,
        name,
        preferred_size,
        min_size,
        max_size,
        media_type
    )
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

-- Quality Definitions for Episodes
INSERT INTO
    quality_definition (
        quality_id,
        name,
        preferred_size,
        min_size,
        max_size,
        media_type
    )
VALUES
    (15, 'HDTV-720p', 995, 10.0, 1000, 'episode'),
    (16, 'WEBDL-720p', 995, 10.0, 1000, 'episode'),
    (17, 'WEBRip-720p', 995, 10.0, 1000, 'episode'),
    (18, 'Bluray-720p', 995, 17.1, 1000, 'episode'),
    (19, 'HDTV-1080p', 995, 15.0, 1000, 'episode'),
    (20, 'WEBDL-1080p', 995, 15.0, 1000, 'episode'),
    (21, 'WEBRip-1080p', 995, 15.0, 1000, 'episode'),
    (22, 'Bluray-1080p', 995, 50.4, 1000, 'episode'),
    (23, 'Remux-1080p', 995, 69.1, 1000, 'episode'),
    (24, 'HDTV-2160p', 995, 25.0, 1000, 'episode'),
    (25, 'WEBDL-2160p', 995, 25.0, 1000, 'episode'),
    (26, 'WEBRip-2160p', 995, 25.0, 1000, 'episode'),
    (27, 'Bluray-2160p', 995, 94.6, 1000, 'episode'),
    (28, 'Remux-2160p', 995, 187.4, 1000, 'episode');

-- Movie Profiles
INSERT
    OR IGNORE INTO quality_profile (id, name, cutoff_quality_id, upgrade_allowed)
VALUES
    (1, 'Standard Definition', 2, TRUE),
    (2, 'High Definition', 8, TRUE),
    (3, 'Ultra High Definition', 13, FALSE);

-- Movie Profile Items
INSERT INTO
    quality_profile_item (profile_id, quality_id)
VALUES
    (1, 1),
    (1, 2),
    (2, 3),
    (2, 4),
    (2, 5),
    (2, 6),
    (2, 7),
    (2, 8),
    (3, 9),
    (3, 10),
    (3, 11),
    (3, 12),
    (3, 13);

-- Episode Profiles
INSERT
    OR IGNORE INTO quality_profile (id, name, cutoff_quality_id, upgrade_allowed)
VALUES
    (4, 'Standard Definition', 16, TRUE),
    (5, 'High Definition', 23, TRUE),
    (6, 'Ultra High Definition', 27, FALSE);

-- Episode Profile Items
INSERT INTO
    quality_profile_item (profile_id, quality_id)
VALUES
    (4, 15),
    (4, 16),
    (5, 17),
    (5, 18),
    (5, 19),
    (5, 20),
    (5, 21),
    (5, 22),
    (5, 23),
    (6, 23),
    (6, 24),
    (6, 25),
    (6, 26),
    (6, 27);