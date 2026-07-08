CREATE TABLE clients (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    private_key TEXT NOT NULL,
    public_key  TEXT NOT NULL,
    address     TEXT NOT NULL,
    created_at  TEXT NOT NULL,
    revoked_at  TEXT
);

CREATE UNIQUE INDEX clients_public_key_idx ON clients (public_key);
CREATE UNIQUE INDEX clients_name_active_idx ON clients (name) WHERE revoked_at IS NULL;
CREATE UNIQUE INDEX clients_address_active_idx ON clients (address) WHERE revoked_at IS NULL;
