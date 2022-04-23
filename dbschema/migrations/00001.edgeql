# Golang port of Overleaf
# Copyright (C) 2022 Jakob Ackermann <das7pad@outlook.com>
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU Affero General Public License as published
# by the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU Affero General Public License for more details.
#
# You should have received a copy of the GNU Affero General Public License
# along with this program.  If not, see <https://www.gnu.org/licenses/>.

CREATE MIGRATION m1bv5sb33s2wzs233ixyniobpzuda2oc5gn3s6prg2kalw5ry7pj5a
    ONTO initial
{
  CREATE ABSTRACT TYPE default::Room;
  CREATE TYPE default::ChatRoom EXTENDING default::Room;
  CREATE TYPE default::EditorConfig {
      CREATE REQUIRED PROPERTY auto_complete -> std::bool;
      CREATE REQUIRED PROPERTY auto_pair_delimiters -> std::bool;
      CREATE REQUIRED PROPERTY font_family -> std::str;
      CREATE REQUIRED PROPERTY font_size -> std::int64;
      CREATE REQUIRED PROPERTY line_height -> std::str;
      CREATE REQUIRED PROPERTY mode -> std::str;
      CREATE REQUIRED PROPERTY overall_theme -> std::str;
      CREATE REQUIRED PROPERTY pdf_viewer -> std::str;
      CREATE REQUIRED PROPERTY spell_check_language -> std::str;
      CREATE REQUIRED PROPERTY syntax_validation -> std::bool;
      CREATE REQUIRED PROPERTY theme -> std::str;
  };
  CREATE TYPE default::Tokens {
      CREATE REQUIRED PROPERTY token_prefix_rw -> std::str {
          CREATE CONSTRAINT std::exclusive;
      };
      CREATE REQUIRED PROPERTY token_ro -> std::str {
          CREATE CONSTRAINT std::exclusive;
      };
      CREATE REQUIRED PROPERTY token_rw -> std::str {
          CREATE CONSTRAINT std::exclusive;
      };
  };
  CREATE TYPE default::Project {
      CREATE REQUIRED PROPERTY public_access_level -> std::str {
          SET default := 'private';
      };
      CREATE LINK tokens -> default::Tokens {
          CREATE CONSTRAINT std::exclusive;
      };
      CREATE REQUIRED PROPERTY active -> std::bool {
          SET default := true;
      };
      CREATE REQUIRED PROPERTY compiler -> std::str;
      CREATE REQUIRED PROPERTY epoch -> std::int64 {
          SET default := 1;
      };
      CREATE REQUIRED PROPERTY image_name -> std::str;
      CREATE PROPERTY last_opened -> std::datetime;
      CREATE REQUIRED PROPERTY last_updated_at -> std::datetime {
          SET default := (std::datetime_of_transaction());
      };
      CREATE REQUIRED PROPERTY name -> std::str;
      CREATE REQUIRED PROPERTY spell_check_language -> std::str;
      CREATE REQUIRED PROPERTY track_changes_state -> std::json {
          SET default := (std::to_json('{}'));
      };
      CREATE REQUIRED PROPERTY version -> std::int64 {
          SET default := 1;
      };
  };
  ALTER TYPE default::Room {
      CREATE REQUIRED LINK project -> default::Project {
          ON TARGET DELETE  DELETE SOURCE;
      };
  };
  ALTER TYPE default::ChatRoom {
      CREATE CONSTRAINT std::exclusive ON (.project);
  };
  CREATE TYPE default::Message {
      CREATE REQUIRED LINK room -> default::Room {
          ON TARGET DELETE  DELETE SOURCE;
      };
      CREATE REQUIRED PROPERTY content -> std::str;
      CREATE REQUIRED PROPERTY created_at -> std::datetime {
          SET default := (std::datetime_of_transaction());
      };
      CREATE PROPERTY edited_at -> std::datetime;
  };
  ALTER TYPE default::Room {
      CREATE MULTI LINK messages := (.<room[IS default::Message]);
  };
  ALTER TYPE default::Project {
      CREATE LINK chat := (SELECT
          .<project[IS default::ChatRoom]
      LIMIT
          1
      );
  };
  CREATE ABSTRACT TYPE default::TreeElement {
      CREATE REQUIRED LINK project -> default::Project {
          ON TARGET DELETE  DELETE SOURCE;
      };
  };
  CREATE ABSTRACT TYPE default::VisibleTreeElement EXTENDING default::TreeElement {
      CREATE REQUIRED PROPERTY deleted_at -> std::datetime {
          SET default := (<std::datetime>'1970-01-01T00:00:00.000000Z');
      };
      CREATE PROPERTY deleted := ((.deleted_at != <std::datetime>'1970-01-01T00:00:00.000000Z'));
      CREATE REQUIRED PROPERTY name -> std::str;
  };
  CREATE ABSTRACT TYPE default::ContentElement EXTENDING default::VisibleTreeElement {
      CREATE REQUIRED PROPERTY size -> std::int64;
  };
  CREATE ABSTRACT TYPE default::FolderLike EXTENDING default::TreeElement {
      CREATE REQUIRED PROPERTY path -> std::str;
      CREATE PROPERTY path_for_join := (('' IF (.path = '') ELSE (.path ++ '/')));
  };
  ALTER TYPE default::VisibleTreeElement {
      CREATE REQUIRED LINK parent -> default::FolderLike {
          ON TARGET DELETE  DELETE SOURCE;
      };
      CREATE CONSTRAINT std::exclusive ON ((.project, .parent, .name, .deleted_at));
  };
  ALTER TYPE default::ContentElement {
      CREATE PROPERTY resolved_path := ((.parent.path_for_join ++ .name));
  };
  CREATE TYPE default::Doc EXTENDING default::ContentElement {
      CREATE REQUIRED PROPERTY in_s3 -> std::bool {
          SET default := false;
      };
      CREATE REQUIRED PROPERTY snapshot -> std::str;
      CREATE REQUIRED PROPERTY version -> std::int64;
  };
  CREATE TYPE default::File EXTENDING default::ContentElement {
      CREATE REQUIRED PROPERTY created_at -> std::datetime {
          SET default := (std::datetime_of_transaction());
      };
      CREATE REQUIRED PROPERTY hash -> std::str;
  };
  CREATE TYPE default::LinkedFileProjectFile EXTENDING default::File {
      CREATE LINK source_element -> default::ContentElement {
          ON TARGET DELETE  ALLOW;
      };
  };
  CREATE TYPE default::LinkedFileProjectOutputFile EXTENDING default::File {
      CREATE LINK source_project -> default::Project {
          ON TARGET DELETE  ALLOW;
      };
      CREATE REQUIRED PROPERTY source_path -> std::str;
  };
  CREATE TYPE default::LinkedFileURL EXTENDING default::File {
      CREATE REQUIRED PROPERTY url -> std::str;
  };
  ALTER TYPE default::Project {
      CREATE MULTI LINK deleted_docs := (SELECT
          .<project[IS default::Doc]
      FILTER
          .deleted
      );
      CREATE MULTI LINK docs := (SELECT
          .<project[IS default::Doc]
      FILTER
          NOT (.deleted)
      );
      CREATE LINK root_doc -> default::Doc {
          ON TARGET DELETE  ALLOW;
      };
      CREATE MULTI LINK files := (SELECT
          .<project[IS default::File]
      FILTER
          NOT (.deleted)
      );
  };
  ALTER TYPE default::FolderLike {
      CREATE MULTI LINK docs := (SELECT
          .<parent[IS default::Doc]
      FILTER
          NOT (.deleted)
      );
      CREATE MULTI LINK files := (SELECT
          .<parent[IS default::File]
      FILTER
          NOT (.deleted)
      );
  };
  CREATE TYPE default::Folder EXTENDING default::VisibleTreeElement, default::FolderLike;
  ALTER TYPE default::FolderLike {
      CREATE MULTI LINK folders := (SELECT
          .<parent[IS default::Folder]
      FILTER
          NOT (.deleted)
      );
  };
  CREATE TYPE default::RootFolder EXTENDING default::FolderLike {
      CREATE CONSTRAINT std::exclusive ON (.project);
  };
  CREATE TYPE default::Features {
      CREATE REQUIRED PROPERTY compile_group -> std::str;
      CREATE REQUIRED PROPERTY compile_timeout -> std::duration;
  };
  CREATE TYPE default::User {
      CREATE REQUIRED LINK editor_config -> default::EditorConfig;
      CREATE REQUIRED LINK features -> default::Features;
      CREATE MULTI LINK contacts -> default::User {
          ON TARGET DELETE  DELETE SOURCE;
          CREATE PROPERTY connections -> std::int64;
          CREATE PROPERTY last_touched -> std::datetime;
      };
      CREATE REQUIRED PROPERTY beta_program -> std::bool {
          SET default := false;
      };
      CREATE REQUIRED PROPERTY epoch -> std::int64 {
          SET default := 1;
      };
      CREATE REQUIRED PROPERTY first_name -> std::str;
      CREATE PROPERTY last_logged_in -> std::datetime;
      CREATE PROPERTY last_login_ip -> std::str;
      CREATE REQUIRED PROPERTY last_name -> std::str;
      CREATE MULTI PROPERTY learned_words -> std::str;
      CREATE REQUIRED PROPERTY login_count -> std::int64 {
          SET default := 0;
      };
      CREATE REQUIRED PROPERTY must_reconfirm -> std::bool {
          SET default := false;
      };
      CREATE REQUIRED PROPERTY password_hash -> std::str;
      CREATE REQUIRED PROPERTY signup_date -> std::datetime {
          SET default := (std::datetime_of_transaction());
      };
  };
  CREATE TYPE default::Email {
      CREATE PROPERTY confirmed_at -> std::datetime;
      CREATE REQUIRED PROPERTY created_at -> std::datetime {
          SET default := (std::datetime_of_transaction());
      };
      CREATE REQUIRED PROPERTY email -> std::str {
          CREATE CONSTRAINT std::exclusive;
      };
  };
  ALTER TYPE default::User {
      CREATE REQUIRED MULTI LINK emails -> default::Email {
          CREATE CONSTRAINT std::exclusive;
      };
      CREATE REQUIRED LINK email -> default::Email {
          CREATE CONSTRAINT std::exclusive;
      };
  };
  ALTER TYPE default::Email {
      CREATE LINK user := (.<emails[IS default::User]);
  };
  CREATE TYPE default::OneTimeToken {
      CREATE REQUIRED LINK email -> default::Email {
          ON TARGET DELETE  DELETE SOURCE;
      };
      CREATE REQUIRED PROPERTY created_at -> std::datetime {
          SET default := (std::datetime_of_transaction());
      };
      CREATE REQUIRED PROPERTY expires_at -> std::datetime;
      CREATE REQUIRED PROPERTY token -> std::str {
          CREATE CONSTRAINT std::exclusive;
      };
      CREATE REQUIRED PROPERTY use -> std::str;
      CREATE PROPERTY used_at -> std::datetime;
  };
  ALTER TYPE default::Project {
      CREATE MULTI LINK folders := (SELECT
          .<project[IS default::Folder]
      FILTER
          NOT (.deleted)
      );
      CREATE MULTI LINK access_ro -> default::User {
          ON TARGET DELETE  ALLOW;
      };
      CREATE MULTI LINK access_rw -> default::User {
          ON TARGET DELETE  ALLOW;
      };
      CREATE MULTI LINK access_token_ro -> default::User {
          ON TARGET DELETE  ALLOW;
      };
      CREATE MULTI LINK access_token_rw -> default::User {
          ON TARGET DELETE  ALLOW;
      };
      CREATE REQUIRED LINK owner -> default::User {
          ON TARGET DELETE  DELETE SOURCE;
      };
      CREATE MULTI LINK min_access_ro := (DISTINCT ((((({.owner} UNION .access_ro) UNION .access_rw) UNION .access_token_ro) UNION .access_token_rw)));
      CREATE MULTI LINK min_access_rw := (DISTINCT ((({.owner} UNION .access_rw) UNION .access_token_rw)));
      CREATE MULTI LINK archived_by -> default::User {
          ON TARGET DELETE  ALLOW;
      };
  };
  CREATE TYPE default::ReviewThread EXTENDING default::Room {
      CREATE LINK resolved_by -> default::User;
      CREATE PROPERTY resolved_at -> std::datetime;
  };
  ALTER TYPE default::Message {
      CREATE REQUIRED LINK user -> default::User;
  };
  CREATE TYPE default::Notification {
      CREATE REQUIRED LINK user -> default::User {
          ON TARGET DELETE  DELETE SOURCE;
      };
      CREATE REQUIRED PROPERTY key -> std::str;
      CREATE CONSTRAINT std::exclusive ON ((.key, .user));
      CREATE REQUIRED PROPERTY expires_at -> std::datetime;
      CREATE REQUIRED PROPERTY message_options -> std::json;
      CREATE REQUIRED PROPERTY template_key -> std::str;
  };
  ALTER TYPE default::User {
      CREATE MULTI LINK projects_owned := (.<owner[IS default::Project]);
      CREATE MULTI LINK projects_ro := (.<access_ro[IS default::Project]);
      CREATE MULTI LINK projects_rw := (.<access_rw[IS default::Project]);
      CREATE MULTI LINK projects_token_ro := (SELECT
          default::Project
      FILTER
          ((default::User IN .access_token_ro) AND (.public_access_level = 'tokenBased'))
      );
      CREATE MULTI LINK projects_token_rw := (SELECT
          default::Project
      FILTER
          ((default::User IN .access_token_rw) AND (.public_access_level = 'tokenBased'))
      );
      CREATE MULTI LINK projects := (DISTINCT (((((.projects_owned UNION .projects_ro) UNION .projects_rw) UNION .projects_token_ro) UNION .projects_token_rw)));
  };
  CREATE TYPE default::ProjectAuditLogEntry {
      CREATE REQUIRED LINK initiator -> default::User;
      CREATE PROPERTY info -> std::json;
      CREATE REQUIRED PROPERTY operation -> std::str;
      CREATE REQUIRED PROPERTY timestamp -> std::datetime {
          SET default := (std::datetime_of_transaction());
      };
  };
  ALTER TYPE default::Project {
      CREATE MULTI LINK audit_log -> default::ProjectAuditLogEntry {
          ON TARGET DELETE  ALLOW;
      };
  };
  CREATE TYPE default::ProjectInvite {
      CREATE REQUIRED LINK project -> default::Project {
          ON TARGET DELETE  DELETE SOURCE;
      };
      CREATE REQUIRED PROPERTY token -> std::str;
      CREATE CONSTRAINT std::exclusive ON ((.project, .token));
      CREATE REQUIRED LINK sending_user -> default::User {
          ON TARGET DELETE  DELETE SOURCE;
      };
      CREATE REQUIRED PROPERTY created_at -> std::datetime {
          SET default := (std::datetime_of_transaction());
      };
      CREATE REQUIRED PROPERTY email -> std::str;
      CREATE REQUIRED PROPERTY expires_at -> std::datetime;
      CREATE REQUIRED PROPERTY privilege_level -> std::str;
  };
  ALTER TYPE default::Project {
      CREATE MULTI LINK invites := (.<project[IS default::ProjectInvite]);
      CREATE REQUIRED LINK last_updated_by -> default::User {
          ON TARGET DELETE  ALLOW;
      };
      CREATE LINK root_folder := (SELECT
          .<project[IS default::RootFolder]
      LIMIT
          1
      );
      CREATE MULTI LINK trashed_by -> default::User {
          ON TARGET DELETE  ALLOW;
      };
  };
  CREATE TYPE default::Tag {
      CREATE MULTI LINK projects -> default::Project {
          ON TARGET DELETE  ALLOW;
      };
      CREATE REQUIRED LINK user -> default::User {
          ON TARGET DELETE  DELETE SOURCE;
      };
      CREATE REQUIRED PROPERTY name -> std::str;
      CREATE CONSTRAINT std::exclusive ON ((.name, .user));
  };
  CREATE TYPE default::SystemMessage {
      CREATE REQUIRED PROPERTY content -> std::str;
  };
  ALTER TYPE default::User {
      CREATE MULTI LINK tags := (.<user[IS default::Tag]);
  };
  CREATE TYPE default::UserAuditLogEntry {
      CREATE REQUIRED LINK initiator -> default::User;
      CREATE PROPERTY info -> std::json;
      CREATE REQUIRED PROPERTY ip_address -> std::str;
      CREATE REQUIRED PROPERTY operation -> std::str;
      CREATE REQUIRED PROPERTY timestamp -> std::datetime {
          SET default := (std::datetime_of_transaction());
      };
  };
  ALTER TYPE default::User {
      CREATE MULTI LINK audit_log -> default::UserAuditLogEntry {
          ON TARGET DELETE  ALLOW;
      };
  };
};
