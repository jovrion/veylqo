-- Links. The internal key is uuid. Timestamps are Unix milliseconds.
-- The partial index covers generated slugs only (ADR-0005).
-- destination_hash is written by the adapter. This file does not hash.
CREATE TABLE links (
    id UUID NOT NULL PRIMARY KEY,
    code TEXT NOT NULL,
    destination TEXT NOT NULL,
    destination_hash TEXT NOT NULL,
    custom BOOLEAN NOT NULL,
    created_at BIGINT NOT NULL,
    expires_at BIGINT,
    deleted_at BIGINT
);

CREATE UNIQUE INDEX links_code ON links (code);

CREATE UNIQUE INDEX links_destination_hash_plain
    ON links (destination_hash) WHERE custom = false;
