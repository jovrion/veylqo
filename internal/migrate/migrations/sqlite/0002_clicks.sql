-- Click counts by public code. No IP, user agent, referrer, token, or
-- destination (ADR-0009). A missing code is not a row.
CREATE TABLE clicks (
    code TEXT NOT NULL PRIMARY KEY,
    click_count INTEGER NOT NULL
);
