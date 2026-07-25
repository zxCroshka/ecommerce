ALTER TABLE userservice.users
    ADD COLUMN is_admin BOOLEAN;

UPDATE userservice.users
SET is_admin = role = 'admin';

ALTER TABLE userservice.users
    ALTER COLUMN is_admin SET DEFAULT FALSE,
    ALTER COLUMN is_admin SET NOT NULL;

ALTER TABLE userservice.users DROP COLUMN role;
