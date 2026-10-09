-- Permissions added to or taken from one person on top of their role (never for administrators).
ALTER TABLE users ADD COLUMN permissions_extra text[] NOT NULL DEFAULT '{}', ADD COLUMN permissions_denied text[] NOT NULL DEFAULT '{}';
