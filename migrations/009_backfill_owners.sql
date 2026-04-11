-- Backfill owner role in room_members for rooms created before role migration
INSERT INTO room_members (room_id, user_id, role)
SELECT id, owner_id, 'owner'
FROM rooms
WHERE owner_id IS NOT NULL
  AND NOT EXISTS (
    SELECT 1 FROM room_members
    WHERE room_members.room_id = rooms.id
      AND room_members.user_id = rooms.owner_id
  )
ON CONFLICT DO NOTHING;
