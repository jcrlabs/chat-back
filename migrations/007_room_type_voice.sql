-- Add 'voice' as a valid room type
ALTER TABLE rooms DROP CONSTRAINT IF EXISTS rooms_type_check;
ALTER TABLE rooms ADD CONSTRAINT rooms_type_check
  CHECK (type IN ('public', 'private', 'dm', 'voice'));
