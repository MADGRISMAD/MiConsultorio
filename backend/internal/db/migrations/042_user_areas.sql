-- The areas (giros) a professional works in. Empty = all the clinic's areas (how every account behaved so far).
ALTER TABLE users ADD COLUMN areas text[] NOT NULL DEFAULT '{}';
