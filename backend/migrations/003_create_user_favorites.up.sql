
Copy

-- 003_create_user_favorites.up.sql
-- Links users to their favorite stocks.
-- References the existing companies table (managed by other services).

CREATE TABLE IF NOT EXISTS user_favorites (
    id          SERIAL PRIMARY KEY,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    company_id  INTEGER NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_user_favorites_user_company UNIQUE (user_id, company_id)
);

CREATE INDEX IF NOT EXISTS ix_user_favorites_user_id ON user_favorites (user_id);
CREATE INDEX IF NOT EXISTS ix_user_favorites_company_id ON user_favorites (company_id);