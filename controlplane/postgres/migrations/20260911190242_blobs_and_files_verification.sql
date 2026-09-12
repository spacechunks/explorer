-- migrate:up transaction:false
ALTER TYPE build_status ADD VALUE IF NOT EXISTS 'FILES_VERIFICATION';
ALTER TYPE build_status ADD VALUE IF NOT EXISTS 'FILES_VERIFICATION_FAILED';

-- migrate:down

