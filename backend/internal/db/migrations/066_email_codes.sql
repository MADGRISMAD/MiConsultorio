-- Verification of the e-mail when signing up: a 6-digit code is sent and the account is only created with it. The code is
-- kept as an HMAC (never in the clear), expires in 10 minutes and dies after 5 wrong tries; sends are counted per hour.
CREATE TABLE email_codes (
    email      text PRIMARY KEY,
    code_hash  text NOT NULL,
    tries      integer NOT NULL DEFAULT 0,
    sent_at    timestamptz NOT NULL,
    sends      timestamptz[] NOT NULL DEFAULT '{}',
    expires_at timestamptz NOT NULL
);
