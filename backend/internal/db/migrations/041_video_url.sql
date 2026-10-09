-- A professional's fixed video-call link (Meet, Zoom...), shown in the appointment e-mails.
ALTER TABLE professional_settings ADD COLUMN video_url text NOT NULL DEFAULT '';
