CREATE TABLE users (
    user_id TEXT PRIMARY KEY,
    username TEXT NOT NULL,
    team_name TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE pull_requests (
    pull_request_id TEXT PRIMARY KEY,
    pull_request_name TEXT NOT NULL,
    author_id TEXT NOT NULL REFERENCES users(user_id),
    status TEXT NOT NULL CHECK (status IN ('OPEN', 'MERGED')),
    assigned_reviewers JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    merged_at TIMESTAMPTZ
);

CREATE INDEX idx_users_team_name ON users(team_name);
CREATE INDEX idx_users_active ON users(is_active) WHERE is_active = true;
CREATE INDEX idx_prs_author ON pull_requests(author_id);
CREATE INDEX idx_prs_status ON pull_requests(status);
CREATE INDEX idx_prs_reviewers_gin ON pull_requests USING gin(assigned_reviewers);