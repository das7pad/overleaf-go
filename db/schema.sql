--  Golang port of Overleaf
--  Copyright (C) 2022 Jakob Ackermann <das7pad@outlook.com>
--
--  This program is free software: you can redistribute it and/or modify
--  it under the terms of the GNU Affero General Public License as published
--  by the Free Software Foundation, either version 3 of the License, or
--  (at your option) any later version.
--
--  This program is distributed in the hope that it will be useful,
--  but WITHOUT ANY WARRANTY; without even the implied warranty of
--  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
--  GNU Affero General Public License for more details.
--
--  You should have received a copy of the GNU Affero General Public License
--  along with this program.  If not, see <https://www.gnu.org/licenses/>.

-- TODO: report duplicate copyright head bug?
-- TODO: back fill ids as needed

CREATE TYPE Features AS
(
  compile_group   TEXT,
  compile_timeout INTERVAL
);


CREATE TYPE EditorConfig AS
(
  auto_complete        BOOLEAN,
  auto_pair_delimiters BOOLEAN,
  syntax_validation    BOOLEAN,
  font_size            SMALLINT,
  font_family          TEXT,
  line_height          TEXT,
  mode                 TEXT,
  overall_theme        TEXT,
  pdf_viewer           TEXT,
  spell_check_language TEXT,
  theme                TEXT
);

CREATE TABLE users
(
  beta_program       BOOLEAN      NOT NULL,
  deleted_at         TIMESTAMP,
  editor_config      EditorConfig NOT NULL,
  email              TEXT         NOT NULL UNIQUE,
  email_confirmed_at TIMESTAMP,
  email_created_at   TIMESTAMP    NOT NULL,
  epoch              INTEGER      NOT NULL,
  features           Features     NOT NULL,
  first_name         TEXT         NOT NULL,
  id                 UUID PRIMARY KEY,
  last_login_at      timestamp,
  last_login_ip      TEXT,
  last_name          TEXT         NOT NULL,
  learned_words      TEXT[],
  login_count        INTEGER      NOT NULL,
  must_reconfirm     BOOLEAN      NOT NULL,
  password_hash      TEXT         NOT NULL,
  signup_date        TIMESTAMP    NOT NULL
);

CREATE TABLE contacts
(
  a            UUID REFERENCES users ON DELETE CASCADE,
  b            UUID REFERENCES users ON DELETE CASCADE,
  connections  INTEGER NOT NULL,
  last_touched TIMESTAMP,
  PRIMARY KEY (a, b)
);

CREATE TABLE user_audit_log
(
  id         UUID PRIMARY KEY,
  info       JSONB,
  initiator  UUID      REFERENCES users ON DELETE SET NULL,
  ip_address TEXT,
  operation  TEXT      NOT NULL,
  timestamp  TIMESTAMP NOT NULL,
  user_id    UUID      NOT NULL REFERENCES users ON DELETE CASCADE
);

CREATE TYPE TreeNodeKind AS ENUM ('doc', 'file', 'folder');

CREATE FUNCTION is_valid_parent(UUID, UUID) RETURNS BOOLEAN AS
$$
SELECT $1 IS NULL OR (SELECT TRUE
                      FROM tree_nodes
                      WHERE id = $1
                        AND project = $2
                        AND kind = 'folder')
$$ LANGUAGE SQL;

CREATE TABLE tree_nodes
(
  deleted_at TIMESTAMP,
  id         UUID PRIMARY KEY,
  kind       TreeNodeKind NOT NULL,
  name       TEXT         NOT NULL,
  parent     UUID REFERENCES tree_nodes ON DELETE CASCADE,
  path       TEXT         NOT NULL,
  project    UUID REFERENCES projects ON DELETE CASCADE,

  -- TODO: check NULL parent behavior, use path instead if not enforced
  UNIQUE (project, parent, name, deleted_at),
  CHECK (is_valid_parent(parent, project)) INITIALLY DEFERRED
);

CREATE TABLE docs
(
  id       UUID REFERENCES tree_nodes ON DELETE CASCADE,
  snapshot TEXT    NOT NULL,
  version  INTEGER NOT NULL
);

CREATE TABLE doc_history
(
  id             UUID PRIMARY KEY,
  doc_id         UUID REFERENCES docs ON DELETE CASCADE,
  user_id        UUID      REFERENCES users ON DELETE SET NULL,
  version        INTEGER   NOT NULL,
  op             JSON      NOT NULL,
  has_big_delete BOOLEAN,
  start_at       TIMESTAMP NOT NULL,
  end_at         TIMESTAMP NOT NULL
);

CREATE TYPE LinkedFileData AS
(
  provider                TEXT,
  source_project_id       UUID,
  source_entity_path      TEXT,
  source_output_file_path TEXT,
  url                     TEXT
);

CREATE TABLE files
(
  id               UUID REFERENCES tree_nodes ON DELETE CASCADE,
  created_at       TIMESTAMP NOT NULL,
  hash             TEXT      NOT NULL,
  linked_file_data LinkedFileData
);

CREATE TABLE projects
(
  compiler             TEXT    NOT NULL,
  deleted_at           TIMESTAMP,
  epoch                INTEGER NOT NULL,
  id                   UUID PRIMARY KEY,
  image_name           TEXT    NOT NULL,
  last_opened_at       TIMESTAMP,
  last_updated_at      TIMESTAMP,
  last_updated_by      UUID    REFERENCES users ON DELETE SET NULL,
  name                 TEXT    NOT NULL,
  owner                UUID REFERENCES users ON DELETE RESTRICT,
  public_access_level  TEXT    NOT NULL,
  root_doc             UUID    REFERENCES docs ON DELETE SET NULL,
  spell_check_language TEXT,
  token_ro             TEXT UNIQUE,
  token_rw             TEXT UNIQUE,
  token_rw_prefix      TEXT UNIQUE,
  tree_version         INTEGER
);

CREATE TABLE project_audit_log
(
  id         UUID PRIMARY KEY,
  info       JSONB,
  initiator  UUID      REFERENCES users ON DELETE SET NULL,
  operation  TEXT      NOT NULL,
  project_id UUID      NOT NULL REFERENCES projects ON DELETE CASCADE,
  timestamp  TIMESTAMP NOT NULL
);

CREATE TABLE project_invites
(
  created_at      TIMESTAMP NOT NULL,
  email           TEXT      NOT NULL,
  expires_at      TIMESTAMP NOT NULL,
  id              UUID PRIMARY KEY,
  privilege_level TEXT      NOT NULL,
  project_id      UUID REFERENCES projects ON DELETE CASCADE,
  sending_user    UUID REFERENCES users ON DELETE CASCADE,
  token           TEXT      NOT NULL,
  UNIQUE (project_id, token)
);

CREATE TABLE one_time_tokens
(
  created_at TIMESTAMP NOT NULL,
  email      TEXT      NOT NULL,
  expires_at TIMESTAMP NOT NULL,
  token      TEXT PRIMARY KEY,
  use        TEXT      NOT NULL,
  used_at    TIMESTAMP
);

CREATE TABLE notifications
(
  expires_at      TIMESTAMP NOT NULL,
  id              UUID PRIMARY KEY,
  key             TEXT      NOT NULL,
  message_options json      NOT NULL,
  template_key    TEXT      NOT NULL,
  user_id         UUID REFERENCES users ON DELETE CASCADE,
  UNIQUE (key, user_id)
);

CREATE TABLE tags
(
  id      UUID PRIMARY KEY,
  name    TEXT NOT NULL,
  user_id UUID REFERENCES users ON DELETE CASCADE,

  UNIQUE (name, user_id)
);

CREATE TABLE tag_entries
(
  project UUID REFERENCES projects ON DELETE CASCADE,
  tag     UUID REFERENCES tags ON DELETE CASCADE,
  PRIMARY KEY (project, tag)
);

CREATE TABLE system_messages
(
  content TEXT NOT NULL,
  id      UUID PRIMARY KEY
);

CREATE TABLE chat_messages
(
  id         UUID PRIMARY KEY,
  project    UUID REFERENCES projects ON DELETE CASCADE,
  content    TEXT      NOT NULL,
  created_at TIMESTAMP NOT NULL,
  user_id    UUID      REFERENCES users ON DELETE SET NULL,
  edited_at  TIMESTAMP
);
