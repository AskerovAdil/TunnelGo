ALTER TABLE clients ADD COLUMN vless_uuid TEXT;

CREATE UNIQUE INDEX clients_vless_uuid_active_idx
  ON clients (vless_uuid)
  WHERE revoked_at IS NULL AND vless_uuid IS NOT NULL;
