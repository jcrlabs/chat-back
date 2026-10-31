ALTER TABLE room_members ADD COLUMN role TEXT NOT NULL DEFAULT 'member'
  CHECK (role IN ('owner','admin','member'));
