-- Add unique 4-digit tag to each user (e.g. alice#0042)
ALTER TABLE users ADD COLUMN IF NOT EXISTS tag VARCHAR(4);

-- Generate unique tags for existing users
DO $$
DECLARE
  u RECORD;
  candidate VARCHAR(4);
BEGIN
  FOR u IN SELECT id FROM users WHERE tag IS NULL LOOP
    LOOP
      candidate := LPAD((floor(random() * 9999 + 1)::int)::text, 4, '0');
      BEGIN
        UPDATE users SET tag = candidate WHERE id = u.id;
        EXIT; -- success, no conflict
      EXCEPTION WHEN unique_violation THEN
        -- retry with a new candidate
      END;
    END LOOP;
  END LOOP;
END $$;

ALTER TABLE users ALTER COLUMN tag SET NOT NULL;
ALTER TABLE users ADD CONSTRAINT users_tag_unique UNIQUE (tag);
