CREATE TABLE clinics (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name         text NOT NULL,
    email        text NOT NULL,
    phone_number text NOT NULL DEFAULT '',
    address      text NOT NULL DEFAULT '',
    image_url    text NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX clinics_email_key ON clinics (lower(email));

CREATE TABLE users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id     uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    username      text NOT NULL,
    password_hash text NOT NULL,
    permissions   text[] NOT NULL DEFAULT '{}',
    created_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (clinic_id, username)
);

CREATE TABLE expedients (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id           uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    curp                text NOT NULL,
    names               text NOT NULL,
    last_names          text NOT NULL,
    sex                 text NOT NULL CHECK (sex IN ('Hombre', 'Mujer')),
    date_of_birth       date NOT NULL,
    education           text NOT NULL DEFAULT '',
    occupation          text NOT NULL DEFAULT '',
    weight              text NOT NULL DEFAULT '',
    clothes_size        text NOT NULL DEFAULT '',
    height              text NOT NULL DEFAULT '',
    ethnicity           text NOT NULL DEFAULT '',
    physical_activity   text NOT NULL DEFAULT '',
    hobbies             text NOT NULL DEFAULT '',
    child               text NOT NULL DEFAULT '',
    diabetes            boolean NOT NULL DEFAULT false,
    rheumatic_diseases  boolean NOT NULL DEFAULT false,
    fractures           boolean NOT NULL DEFAULT false,
    allergies           boolean NOT NULL DEFAULT false,
    layed               boolean NOT NULL DEFAULT false,
    contractures        boolean NOT NULL DEFAULT false,
    cancer              boolean NOT NULL DEFAULT false,
    accidents           boolean NOT NULL DEFAULT false,
    transfusions        boolean NOT NULL DEFAULT false,
    cardiopathies       boolean NOT NULL DEFAULT false,
    surgeries           boolean NOT NULL DEFAULT false,
    tabaquism           boolean NOT NULL DEFAULT false,
    alcoholism          boolean NOT NULL DEFAULT false,
    automedication      boolean NOT NULL DEFAULT false,
    drug_use            boolean NOT NULL DEFAULT false,
    pregnant            boolean NOT NULL DEFAULT false,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    UNIQUE (clinic_id, curp)
);

-- Appointments reference patients by CURP without a foreign key: a visit can be
-- booked for someone who has no medical record yet.
CREATE TABLE appointments (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id   uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    curp        text NOT NULL,
    names       text NOT NULL,
    last_names  text NOT NULL,
    date        date NOT NULL,
    start_hour  time NOT NULL,
    end_hour    time NOT NULL,
    details     text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CHECK (end_hour > start_hour)
);
CREATE INDEX appointments_clinic_date_idx ON appointments (clinic_id, date, start_hour);
