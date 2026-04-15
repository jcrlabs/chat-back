-- 011_room_reads.sql
-- Tracks last time a user read each room for unread badge counts.

CREATE TABLE room_reads (
    user_id      UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    room_id      UUID        NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    last_read_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, room_id)
);

CREATE INDEX room_reads_room_idx ON room_reads(room_id);
