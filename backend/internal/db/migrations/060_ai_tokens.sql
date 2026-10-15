-- Real consumption of the AI allowance: tokens the model billed, per clinic and month (no text is kept).
ALTER TABLE magic_usage
    ADD COLUMN prompt_tokens bigint NOT NULL DEFAULT 0,
    ADD COLUMN output_tokens bigint NOT NULL DEFAULT 0;
