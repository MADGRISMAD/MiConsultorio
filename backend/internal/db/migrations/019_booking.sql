-- Online booking and reminders: where consent for reminders comes from when the patient is not
-- registered yet, how long before the visit the patient may still cancel, and whether the public
-- page shows prices.
ALTER TABLE appointments
    ADD COLUMN reminders_consent    boolean NOT NULL DEFAULT false, -- the person agreed to reminders when booking online
    ADD COLUMN privacy_accepted_at  timestamptz;                    -- aviso de privacidad accepted when booking online

ALTER TABLE agenda_settings
    ADD COLUMN cancel_min_hours      integer NOT NULL DEFAULT 2 CHECK (cancel_min_hours BETWEEN 0 AND 720),
    ADD COLUMN booking_show_prices   boolean NOT NULL DEFAULT false;

CREATE INDEX appointments_contact_idx ON appointments (clinic_id, lower(email)) WHERE email <> '';
