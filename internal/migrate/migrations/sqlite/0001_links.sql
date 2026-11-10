-- Links. Timestamps are Unix milliseconds. custom is 0 or 1.
-- The partial index covers generated slugs only (ADR-0005).
-- destination_hash is written by the adapter. This file does not hash.
CREATE TABLE links (
    id TEXT NOT NULL PRIMARY KEY,
    code TEXT NOT NULL,
    destination TEXT NOT NULL,
    destination_hash TEXT NOT NULL,
    custom INTEGER NOT NULL CHECK (custom IN (0, 1)),
    created_at INTEGER NOT NULL,
    expires_at INTEGER,
    deleted_at INTEGER
);

CREATE UNIQUE INDEX links_code ON links (code);

CREATE UNIQUE INDEX links_destination_hash_plain
    ON links (destination_hash) WHERE custom = 0;
