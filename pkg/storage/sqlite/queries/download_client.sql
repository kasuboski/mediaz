-- name: CreateDownloadClient :one
INSERT INTO download_client (type, implementation, scheme, host, port, api_key)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: GetDownloadClient :one
SELECT
    id,
    type,
    implementation,
    scheme,
    host,
    port,
    api_key
FROM download_client
WHERE id = ?;

-- name: ListDownloadClients :many
SELECT
    id,
    type,
    implementation,
    scheme,
    host,
    port,
    api_key
FROM download_client
ORDER BY id ASC;

-- name: UpdateDownloadClient :exec
UPDATE download_client
SET
    type = ?,
    implementation = ?,
    scheme = ?,
    host = ?,
    port = ?,
    api_key = ?
WHERE id = ?;

-- name: DeleteDownloadClient :exec
DELETE FROM download_client WHERE id = ?;
