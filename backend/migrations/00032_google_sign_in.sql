-- Stage 29 (D-171): Google sign-in for contributors. Off until
-- GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET are set.

-- +goose Up
-- One Google account per user and one user per Google account. subject is
-- Google's stable account ID ("sub"); email is what Google reported at the
-- last sign-in, kept to show the person which account is linked.
CREATE TABLE user_identities (
  id           uuid PRIMARY KEY DEFAULT uuid_v7(),
  user_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  provider     text NOT NULL CHECK (provider IN ('google')),
  subject      text NOT NULL CHECK (length(subject) BETWEEN 1 AND 255),
  email        text NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  last_used_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (provider, subject),
  UNIQUE (user_id, provider)
);

-- A sign-in in progress: created when the browser leaves for Google, used
-- once when it comes back, gone after 10 minutes. The browser holds the
-- state value in a cookie; only its hash is stored.
CREATE TABLE oauth_states (
  id            uuid PRIMARY KEY DEFAULT uuid_v7(),
  state_hash    bytea NOT NULL UNIQUE,
  code_verifier text NOT NULL,
  nonce         text NOT NULL,
  intent        text NOT NULL CHECK (intent IN ('signin', 'signup')),
  consents      jsonb NOT NULL DEFAULT '{}',
  expires_at    timestamptz NOT NULL,
  created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX oauth_states_expires_idx ON oauth_states (expires_at);

GRANT SELECT, INSERT, UPDATE, DELETE ON user_identities, oauth_states TO mezai_app;

-- +goose Down
DROP TABLE oauth_states;
DROP TABLE user_identities;
