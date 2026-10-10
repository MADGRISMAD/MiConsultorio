-- A private feed of each professional's appointments, to subscribe to from Google Calendar (or any calendar app).
-- It only goes out: Caresia never reads the person's own calendar.
ALTER TABLE users
    ADD COLUMN calendar_token text UNIQUE,                           -- the secret in the feed address; NULL = feed off
    ADD COLUMN calendar_show_names boolean NOT NULL DEFAULT false;   -- patient names in the events (off: only "Cita")
