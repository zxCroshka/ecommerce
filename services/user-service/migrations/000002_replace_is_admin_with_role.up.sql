DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'userservice'
          AND table_name = 'users'
          AND column_name = 'is_admin'
    ) THEN
        ALTER TABLE userservice.users ADD COLUMN role TEXT;

        UPDATE userservice.users
        SET role = CASE WHEN is_admin THEN 'admin' ELSE 'customer' END;

        ALTER TABLE userservice.users
            ALTER COLUMN role SET DEFAULT 'customer',
            ALTER COLUMN role SET NOT NULL;

        ALTER TABLE userservice.users DROP COLUMN is_admin;
    END IF;
END $$;

ALTER TABLE userservice.users
    DROP CONSTRAINT IF EXISTS users_role_check;

ALTER TABLE userservice.users
    ADD CONSTRAINT users_role_check CHECK (role IN ('customer', 'admin'));
