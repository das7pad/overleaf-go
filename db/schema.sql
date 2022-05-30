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
  id           UUID PRIMARY KEY,
  info         JSONB,
  initiator_id UUID      REFERENCES users ON DELETE SET NULL,
  ip_address   TEXT,
  operation    TEXT      NOT NULL,
  timestamp    TIMESTAMP NOT NULL,
  user_id      UUID      NOT NULL REFERENCES users ON DELETE CASCADE
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
  owner_id             UUID REFERENCES users ON DELETE RESTRICT,
  public_access_level  TEXT    NOT NULL,
  spell_check_language TEXT    NOT NULL,
  token_ro             TEXT UNIQUE,
  token_rw             TEXT UNIQUE,
  token_rw_prefix      TEXT UNIQUE,
  tree_version         INTEGER NOT NULL
);

CREATE TABLE project_members
(
  project_id    UUID    NOT NULL REFERENCES projects ON DELETE CASCADE,
  user_id       UUID    NOT NULL REFERENCES users ON DELETE CASCADE,
  can_write     BOOLEAN NOT NULL,
  is_token_user BOOLEAN NOT NULL,
  archived      BOOLEAN NOT NULL,
  trashed       BOOLEAN NOT NULL,

  PRIMARY KEY (project_id, user_id)
);

CREATE TABLE project_audit_log
(
  id           UUID PRIMARY KEY,
  info         JSONB,
  initiator_id UUID      REFERENCES users ON DELETE SET NULL,
  operation    TEXT      NOT NULL,
  project_id   UUID      NOT NULL REFERENCES projects ON DELETE CASCADE,
  timestamp    TIMESTAMP NOT NULL
);

CREATE TABLE project_invites
(
  created_at      TIMESTAMP NOT NULL,
  email           TEXT      NOT NULL,
  expires_at      TIMESTAMP NOT NULL,
  id              UUID PRIMARY KEY,
  privilege_level TEXT      NOT NULL,
  project_id      UUID REFERENCES projects ON DELETE CASCADE,
  sending_user_id UUID REFERENCES users ON DELETE CASCADE,
  token           TEXT      NOT NULL,
  UNIQUE (project_id, token)
);

CREATE TYPE TreeNodeKind AS ENUM ('doc', 'file', 'folder');

CREATE TABLE tree_nodes
(
  deleted_at TIMESTAMP,
  id         UUID PRIMARY KEY,
  kind       TreeNodeKind NOT NULL,
  name       TEXT         NOT NULL,
  parent_id  UUID         NOT NULL REFERENCES tree_nodes ON DELETE CASCADE,
  path       TEXT         NOT NULL,
  project_id UUID         NOT NULL REFERENCES projects ON DELETE CASCADE,

  -- TODO: check NULL parent behavior, use path instead if not enforced
  UNIQUE (project_id, parent_id, name, deleted_at)
);

CREATE FUNCTION is_folder(project UUID, folder UUID) RETURNS BOOLEAN AS
$$
SELECT TRUE
FROM tree_nodes
WHERE id = $2
  AND project_id = $1
  AND kind = 'folder'
$$ LANGUAGE SQL;

ALTER TABLE tree_nodes
  ADD CONSTRAINT tree_nodes_parent_check
    -- is root folder (folder w/o parent) or parent is folder
    CHECK (
        (parent_id IS NULL AND kind = 'folder')
        OR
        is_folder(project_id, parent_id)
      );

CREATE TABLE docs
(
  id       UUID PRIMARY KEY REFERENCES tree_nodes ON DELETE CASCADE,
  snapshot TEXT    NOT NULL,
  version  INTEGER NOT NULL
);

ALTER TABLE projects
  ADD COLUMN root_doc_id UUID REFERENCES docs ON DELETE SET NULL;

ALTER TABLE projects
  ADD COLUMN root_folder_id UUID REFERENCES tree_nodes
    CHECK (is_folder(id, root_folder_id));

CREATE TABLE doc_history
(
  id             UUID PRIMARY KEY,
  doc_id         UUID      NOT NULL REFERENCES docs ON DELETE CASCADE,
  user_id        UUID      REFERENCES users ON DELETE SET NULL,
  version        INTEGER   NOT NULL,
  op             JSON      NOT NULL,
  has_big_delete BOOLEAN   NOT NULL,
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
  id               UUID PRIMARY KEY REFERENCES tree_nodes ON DELETE CASCADE,
  created_at       TIMESTAMP NOT NULL,
  hash             TEXT      NOT NULL,
  linked_file_data LinkedFileData
);

CREATE TABLE one_time_tokens
(
  created_at TIMESTAMP NOT NULL,
  email      TEXT      NOT NULL,
  expires_at TIMESTAMP NOT NULL,
  token      TEXT PRIMARY KEY,
  use        TEXT      NOT NULL,
  used_at    TIMESTAMP,
  user_id    UUID      NOT NULL REFERENCES users ON DELETE CASCADE
);

CREATE TABLE notifications
(
  expires_at      TIMESTAMP NOT NULL,
  id              UUID PRIMARY KEY,
  key             TEXT      NOT NULL,
  message_options json      NOT NULL,
  template_key    TEXT      NOT NULL,
  user_id         UUID      NOT NULL REFERENCES users ON DELETE CASCADE,
  UNIQUE (key, user_id)
);

CREATE TABLE tags
(
  id      UUID PRIMARY KEY,
  name    TEXT NOT NULL,
  user_id UUID NOT NULL REFERENCES users ON DELETE CASCADE,

  UNIQUE (name, user_id)
);

CREATE TABLE tag_entries
(
  project_id UUID NOT NULL REFERENCES projects ON DELETE CASCADE,
  tag_id     UUID NOT NULL REFERENCES tags ON DELETE CASCADE,
  PRIMARY KEY (project_id, tag_id)
);

CREATE TABLE system_messages
(
  content TEXT NOT NULL,
  id      UUID PRIMARY KEY
);

CREATE TABLE chat_messages
(
  id         UUID PRIMARY KEY,
  project_id UUID      NOT NULL REFERENCES projects ON DELETE CASCADE,
  content    TEXT      NOT NULL,
  created_at TIMESTAMP NOT NULL,
  user_id    UUID      REFERENCES users ON DELETE SET NULL,
  edited_at  TIMESTAMP
);
