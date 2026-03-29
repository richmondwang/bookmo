-- 012_participant_eligibility.up.sql
-- Adds allows_participants to categories and services.
-- NULL = inherit / not set. Resolution priority: service → category → parent → false.

ALTER TABLE categories ADD COLUMN IF NOT EXISTS allows_participants BOOLEAN DEFAULT NULL;
ALTER TABLE services   ADD COLUMN IF NOT EXISTS allows_participants BOOLEAN DEFAULT NULL;

-- Seed category defaults for the Philippines market taxonomy.
-- Sports & Fitness and Recreation allow participants; Beauty & Services do not.
-- Adjust slugs to match your actual seeded category data.
UPDATE categories SET allows_participants = true
WHERE slug IN (
    'sports-fitness',
    'basketball-court',
    'tennis-court',
    'badminton-court',
    'swimming-pool',
    'gym',
    'volleyball-court',
    'recreation',
    'billiards',
    'bowling',
    'karaoke-room',
    'gaming-cafe',
    'events-venues',
    'function-hall',
    'meeting-room',
    'photo-studio',
    'recording-studio'
);

UPDATE categories SET allows_participants = false
WHERE slug IN (
    'beauty-wellness',
    'hair-salon',
    'nail-salon',
    'spa-massage',
    'barbershop',
    'services',
    'tutoring',
    'music-lessons',
    'driving-school'
);
