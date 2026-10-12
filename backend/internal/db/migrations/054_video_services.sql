-- Videoconsulta: a service can be given in person, by video call, or both (only in the giros where it is prudent).
ALTER TABLE catalog_items ADD COLUMN modality text NOT NULL DEFAULT 'presencial' CHECK (modality IN ('presencial', 'videollamada', 'ambas'));
-- NULL is how appointments were before: the professional's fixed video link (if any) is shown. 'presencial' hides it, 'videollamada' shows
-- the link of the visit (the professional's own, or a private room made for this appointment).
ALTER TABLE appointments
    ADD COLUMN modality  text CHECK (modality IN ('presencial', 'videollamada')),
    ADD COLUMN video_url text NOT NULL DEFAULT '';
