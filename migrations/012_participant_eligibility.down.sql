-- 012_participant_eligibility.down.sql

ALTER TABLE services   DROP COLUMN IF EXISTS allows_participants;
ALTER TABLE categories DROP COLUMN IF EXISTS allows_participants;
