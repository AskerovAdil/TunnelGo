ALTER TABLE clients ADD COLUMN ss_password TEXT;
ALTER TABLE clients ADD COLUMN ss_port INTEGER;

CREATE UNIQUE INDEX clients_ss_port_active_idx ON clients (ss_port) WHERE revoked_at IS NULL AND ss_port IS NOT NULL;
