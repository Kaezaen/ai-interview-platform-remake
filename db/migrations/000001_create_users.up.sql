-- Accounts of people who practise interviews.
--
-- Columns are added by later migrations when a feature needs them; a new
-- migration is cheap, an unused column is not.
BEGIN;

CREATE TABLE users (
    -- uuidv7() (new in PostgreSQL 18) produces time-ordered ids: new rows
    -- land at the end of the primary-key index instead of at random pages,
    -- which keeps inserts cheap as the table grows. Trade-off: an id
    -- reveals roughly when the account was created.
    id            uuid        PRIMARY KEY DEFAULT uuidv7(),
    email         text        NOT NULL,
    display_name  text        NOT NULL,
    password_hash text        NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),

    -- The application normalises email (trim + lowercase) before writing.
    -- This check turns that convention into a guarantee: no code path can
    -- store "Alice@Example.com" next to "alice@example.com".
    CONSTRAINT users_email_normalized
        CHECK (email = lower(btrim(email))),
    CONSTRAINT users_email_length
        CHECK (char_length(email) BETWEEN 3 AND 254),
    CONSTRAINT users_display_name_trimmed
        CHECK (display_name = btrim(display_name) AND char_length(display_name) BETWEEN 1 AND 100),
    -- Only argon2id hashes in PHC string format are accepted, so a bug can
    -- never store a plaintext password.
    CONSTRAINT users_password_hash_argon2id
        CHECK (password_hash LIKE '$argon2id$%'),

    -- The application maps a violation of this constraint (by name) to
    -- EMAIL_ALREADY_REGISTERED.
    CONSTRAINT users_email_key UNIQUE (email)
);

COMMIT;
