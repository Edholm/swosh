-- When the public last opened the link: its page, redirect or QR code.
-- Visits by logged-in admins don't count. NULL means not since this column
-- was added.
ALTER TABLE swosh ADD COLUMN last_accessed_at timestamptz;
