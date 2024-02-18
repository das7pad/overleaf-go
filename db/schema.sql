--  Golang port of Overleaf
--  Copyright (C) 2022-2023 Jakob Ackermann <das7pad@outlook.com>
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

CREATE TABLE users
(
  beta_program       BOOLEAN   NOT NULL,
  created_at         TIMESTAMP NOT NULL,
  deleted_at         TIMESTAMP NULL,
  editor_config      JSONB     NOT NULL,
  email              TEXT      NOT NULL UNIQUE,
  email_confirmed_at TIMESTAMP NULL,
  email_created_at   TIMESTAMP NOT NULL,
  epoch              INTEGER   NOT NULL,
  features           JSONB     NOT NULL,
  first_name         TEXT      NOT NULL,
  id                 UUID      NOT NULL PRIMARY KEY,
  last_login_at      TIMESTAMP NULL,
  last_login_ip      TEXT      NULL,
  last_name          TEXT      NOT NULL,
  learned_words      TEXT[]    NOT NULL,
  login_count        INTEGER   NOT NULL,
  must_reconfirm     BOOLEAN   NOT NULL,
  password_hash      TEXT      NOT NULL
);

CREATE TABLE contacts
(
  a               UUID      NOT NULL REFERENCES users ON DELETE CASCADE,
  b               UUID      NOT NULL REFERENCES users ON DELETE CASCADE,
  connections     INTEGER   NOT NULL,
  last_touched_at TIMESTAMP NOT NULL,

  PRIMARY KEY (a, b)
);

CREATE TABLE user_audit_log
(
  created_at   TIMESTAMP NOT NULL,
  id           UUID      NOT NULL PRIMARY KEY,
  info         JSONB     NULL,
  initiator_id UUID      NULL REFERENCES users ON DELETE SET NULL,
  ip_address   TEXT      NULL,
  operation    TEXT      NOT NULL,
  user_id      UUID      NOT NULL REFERENCES users ON DELETE CASCADE
);

CREATE TYPE PublicAccessLevel AS ENUM ('private', 'tokenBased');

CREATE TABLE projects
(
  compiler             TEXT              NOT NULL,
  created_at           TIMESTAMP         NOT NULL,
  deleted_at           TIMESTAMP         NULL,
  epoch                INTEGER           NOT NULL,
  id                   UUID              NOT NULL PRIMARY KEY,
  image_name           TEXT              NOT NULL,
  last_opened_at       TIMESTAMP         NULL,
  last_updated_at      TIMESTAMP         NULL,
  last_updated_by      UUID              NULL REFERENCES users ON DELETE SET NULL,
  name                 TEXT              NOT NULL,
  owner_id             UUID              NOT NULL REFERENCES users ON DELETE RESTRICT,
  public_access_level  PublicAccessLevel NOT NULL,
  spell_check_language TEXT              NOT NULL,
  token_ro             TEXT              NULL UNIQUE,
  token_rw             TEXT              NULL, -- implicit UNIQUE via token_rw_prefix
  token_rw_prefix      TEXT              NULL UNIQUE,
  tree_version INTEGER NOT NULL                -- TODO: rename to version, it is used for cache invalidation of ForBootstrapWS in real-time
);

CREATE TYPE AccessSource AS ENUM ('token', 'invite', 'owner');
CREATE TYPE PrivilegeLevel AS ENUM ('readOnly', 'readAndWrite', 'owner');

CREATE TABLE project_members
(
  project_id      UUID           NOT NULL REFERENCES projects ON DELETE CASCADE,
  user_id         UUID           NOT NULL REFERENCES users ON DELETE CASCADE,
  access_source   AccessSource   NOT NULL,
  privilege_level PrivilegeLevel NOT NULL,
  archived        BOOLEAN        NOT NULL,
  trashed         BOOLEAN        NOT NULL,

  PRIMARY KEY (project_id, user_id)
);
CREATE INDEX ON project_members (user_id);

CREATE TABLE project_audit_log
(
  created_at   TIMESTAMP NOT NULL,
  id           UUID      NOT NULL PRIMARY KEY,
  info         JSONB     NULL,
  initiator_id UUID      NULL REFERENCES users ON DELETE SET NULL,
  operation    TEXT      NOT NULL,
  project_id   UUID      NOT NULL REFERENCES projects ON DELETE CASCADE
);

CREATE TABLE project_invites
(
  created_at      TIMESTAMP      NOT NULL,
  email           TEXT           NOT NULL,
  expires_at      TIMESTAMP      NOT NULL,
  id              UUID           NOT NULL PRIMARY KEY,
  privilege_level PrivilegeLevel NOT NULL,
  project_id      UUID           NOT NULL REFERENCES projects ON DELETE CASCADE,
  sending_user_id UUID           NOT NULL REFERENCES users ON DELETE CASCADE,
  token           TEXT           NOT NULL,

  UNIQUE (project_id, token)
);
CREATE INDEX ON project_invites (token);

CREATE TYPE TreeNodeKind AS ENUM ('doc', 'file', 'folder');

CREATE TABLE tree_nodes
(
  created_at TIMESTAMP    NOT NULL,
  deleted_at TIMESTAMP    NOT NULL,
  id         UUID         NOT NULL PRIMARY KEY,
  kind       TreeNodeKind NOT NULL,
  parent_id  UUID         NULL REFERENCES tree_nodes ON DELETE CASCADE,
  path       TEXT         NOT NULL,
  project_id UUID         NOT NULL REFERENCES projects ON DELETE CASCADE,

  -- Assumption: creation and deletion of nodes takes >1 microsecond,
  --  which is the resolution of deleted_at.
  UNIQUE (project_id, deleted_at, path)
);

CREATE UNIQUE INDEX ON tree_nodes (project_id) WHERE (parent_id IS NULL);

CREATE FUNCTION is_tree_node_kind(node UUID, k TreeNodeKind)
  RETURNS BOOLEAN
  LANGUAGE SQL
  STABLE
  STRICT
  PARALLEL SAFE
AS
$$
SELECT TRUE
FROM tree_nodes
WHERE id = node
  AND kind = k
LIMIT 1
$$;

CREATE FUNCTION is_tree_node_kind_in(project UUID, node UUID, k TreeNodeKind)
  RETURNS BOOLEAN
  LANGUAGE SQL
  STABLE
  STRICT
  PARALLEL SAFE
AS
$$
SELECT TRUE
FROM tree_nodes
WHERE id = node
  AND project_id = project
  AND kind = k
LIMIT 1
$$;

CREATE FUNCTION is_tree_node_root(project UUID, node UUID)
  RETURNS BOOLEAN
  LANGUAGE SQL
  STABLE
  STRICT
  PARALLEL SAFE
AS
$$
SELECT TRUE
FROM tree_nodes
WHERE id = node
  AND project_id = project
  AND kind = 'folder'
  AND parent_id IS NULL
LIMIT 1
$$;

ALTER TABLE projects
  ADD COLUMN root_folder_id UUID NULL REFERENCES tree_nodes
    CHECK (
      -- Check field is not yet set OR enforce is actual root folder.
        root_folder_id IS NULL
        OR is_tree_node_root(id, root_folder_id)
      );

ALTER TABLE tree_nodes
  ADD CONSTRAINT tree_nodes_parent_check
    CHECK (
      -- Check we are root folder (folder w/o parent)
      --  OR parent is folder in the same project.
        (parent_id IS NULL AND kind = 'folder')
        OR
        is_tree_node_kind_in(project_id, parent_id, 'folder')
      );

CREATE TABLE docs
(
  id       UUID    NOT NULL PRIMARY KEY REFERENCES tree_nodes ON DELETE CASCADE,
  snapshot TEXT    NOT NULL,
  version  INTEGER NOT NULL,

  CHECK (is_tree_node_kind(id, 'doc'))
);

ALTER TABLE projects
  ADD COLUMN root_doc_id UUID NULL REFERENCES docs ON DELETE SET NULL;

CREATE TABLE doc_history
(
  id             UUID      NOT NULL PRIMARY KEY,
  doc_id         UUID      NOT NULL REFERENCES docs ON DELETE CASCADE,
  user_id        UUID      NULL REFERENCES users ON DELETE SET NULL,
  version        INTEGER   NOT NULL,
  op             JSON      NOT NULL,
  has_big_delete BOOLEAN   NOT NULL,
  start_at       TIMESTAMP NOT NULL,
  end_at         TIMESTAMP NOT NULL
);
CREATE UNIQUE INDEX ON doc_history (doc_id, version DESC);

CREATE TABLE files
(
  id               UUID    NOT NULL PRIMARY KEY REFERENCES tree_nodes ON DELETE CASCADE,
  hash             TEXT    NOT NULL,
  linked_file_data JSON    NULL,
  size             INTEGER NOT NULL,
  pending          BOOLEAN NOT NULL,

  CHECK (is_tree_node_kind(id, 'file'))
);

CREATE INDEX ON files (pending) WHERE (pending = TRUE);

CREATE TABLE one_time_tokens
(
  created_at TIMESTAMP NOT NULL,
  email      TEXT      NOT NULL,
  expires_at TIMESTAMP NOT NULL,
  token      TEXT      NOT NULL PRIMARY KEY,
  use        TEXT      NOT NULL,
  used_at    TIMESTAMP NULL,
  user_id    UUID      NOT NULL REFERENCES users ON DELETE CASCADE
);

CREATE TABLE notifications
(
  expires_at      TIMESTAMP NOT NULL,
  id              UUID      NOT NULL PRIMARY KEY,
  key             TEXT      NOT NULL UNIQUE,
  message_options json      NOT NULL,
  template_key    TEXT      NOT NULL,
  user_id         UUID      NOT NULL REFERENCES users ON DELETE CASCADE
);
CREATE INDEX ON notifications (user_id);

CREATE TABLE tags
(
  id      UUID NOT NULL PRIMARY KEY,
  name    TEXT NOT NULL,
  user_id UUID NOT NULL REFERENCES users ON DELETE CASCADE,

  UNIQUE (user_id, name)
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
  id      UUID NOT NULL PRIMARY KEY
);

CREATE TABLE chat_messages
(
  id         UUID      NOT NULL PRIMARY KEY,
  project_id UUID      NOT NULL REFERENCES projects ON DELETE CASCADE,
  content    TEXT      NOT NULL,
  created_at TIMESTAMP NOT NULL,
  user_id    UUID      NULL REFERENCES users ON DELETE SET NULL
);
CREATE INDEX ON chat_messages (project_id, created_at DESC);
