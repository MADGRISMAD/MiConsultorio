// Limits that the server also enforces. Keep them equal to the backend constants (db.PasswordMinRunes / PasswordMaxBytes and
// api.MaxHighlights / MaxFAQs): a backend test reads this file and fails when they differ.

export const PASSWORD_MIN = 8;
export const PASSWORD_MAX = 72;
export const MAX_HIGHLIGHTS = 6;
export const MAX_FAQS = 12;
