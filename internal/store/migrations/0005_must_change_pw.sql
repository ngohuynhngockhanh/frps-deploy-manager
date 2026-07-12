-- Force a password change for accounts created with a known/temp password.
-- Applies to freshly-seeded admins and admin-created managers (set in code);
-- the existing admin is not hard-forced (avoid lockout) but can change its
-- password anytime at /change-password.
ALTER TABLE app_users ADD COLUMN must_change_password INTEGER NOT NULL DEFAULT 0;
