-- +goose Up
-- Split the freeform "role" into a structured NSF 1030 role and a descriptive title.

-- Add title column (descriptive job title: Investigator, Programmer, etc.)
ALTER TABLE personnel ADD COLUMN title TEXT NOT NULL DEFAULT '';

-- Copy current role text into title, then normalize role to NSF 1030 category codes.
UPDATE personnel SET
  title = role,
  role = CASE
    WHEN lower(trim(role)) IN ('pi', 'principal investigator') THEN 'pi'
    WHEN lower(trim(role)) IN ('co-pi', 'copi', 'co_pi', 'co-principal investigator') THEN 'co_pi'
    WHEN lower(trim(role)) LIKE '%subaward%pi%' OR lower(trim(role)) = 'sub-pi' THEN 'subaward_pi'
    WHEN lower(trim(role)) LIKE '%postdoc%' OR lower(trim(role)) LIKE '%post-doc%' OR lower(trim(role)) LIKE '%post doc%' THEN 'postdoc'
    WHEN lower(trim(role)) LIKE '%graduate%' OR lower(trim(role)) LIKE '%grad student%' OR lower(trim(role)) LIKE '%phd%' THEN 'graduate_student'
    WHEN lower(trim(role)) LIKE '%undergraduate%' OR lower(trim(role)) LIKE '%undergrad%' THEN 'undergraduate_student'
    WHEN lower(trim(role)) LIKE '%clerical%' OR lower(trim(role)) LIKE '%secretary%' THEN 'clerical'
    WHEN lower(trim(role)) IN ('senior personnel', 'faculty', 'senior', 'professor') THEN 'senior_personnel'
    WHEN lower(trim(role)) IN ('other professional', 'programmer', 'technician', 'research scientist') THEN 'other_professional'
    ELSE 'other'
  END;

-- +goose Down
ALTER TABLE personnel DROP COLUMN title;
