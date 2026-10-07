-- Point works like MiTiendita: the clinic registers one terminal; charges are Orders; voiding refunds them.
ALTER TABLE mp_accounts
    ADD COLUMN terminal_id    text NOT NULL DEFAULT '',
    ADD COLUMN terminal_label text NOT NULL DEFAULT '';
ALTER TABLE mp_charges ADD COLUMN refunded_at timestamptz;
