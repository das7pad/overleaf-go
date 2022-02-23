CREATE MIGRATION m1b7iy26ufukkc7xjgj4fmclvovog3caekvvegsdhx2nt4ggynl7ba
    ONTO initial
{
  CREATE ABSTRACT TYPE default::Room;
  CREATE TYPE default::ChatRoom EXTENDING default::Room;
  CREATE TYPE default::Email {
      CREATE PROPERTY confirmed_at -> std::datetime;
      CREATE REQUIRED PROPERTY created_at -> std::datetime {
          SET default := (std::datetime_of_transaction());
      };
      CREATE REQUIRED PROPERTY email -> std::str {
          CREATE CONSTRAINT std::exclusive;
      };
  };
  CREATE TYPE default::Features {
      CREATE REQUIRED PROPERTY compile_group -> std::str;
      CREATE REQUIRED PROPERTY compile_timeout -> std::duration;
      CREATE REQUIRED PROPERTY name -> std::str {
          CREATE CONSTRAINT std::exclusive;
      };
      CREATE REQUIRED PROPERTY versioning -> std::bool;
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
      CREATE REQUIRED LINK tokens -> default::Tokens;
      CREATE REQUIRED PROPERTY compiler -> std::str;
      CREATE REQUIRED PROPERTY epoch -> std::int64 {
          SET default := 1;
      };
      CREATE REQUIRED PROPERTY image_name -> std::str;
      CREATE PROPERTY last_opened -> std::datetime;
      CREATE PROPERTY last_updated_at -> std::datetime;
      CREATE REQUIRED PROPERTY name -> std::str;
      CREATE PROPERTY public_access_level -> std::str;
      CREATE REQUIRED PROPERTY spell_check_language -> std::str;
      CREATE REQUIRED PROPERTY version -> std::int64 {
          SET default := 1;
      };
  };
  ALTER TYPE default::Room {
      CREATE REQUIRED LINK project -> default::Project {
          ON TARGET DELETE  DELETE SOURCE;
      };
  };
  CREATE ABSTRACT TYPE default::TreeElement {
      CREATE REQUIRED LINK project -> default::Project {
          ON TARGET DELETE  DELETE SOURCE;
      };
  };
  CREATE ABSTRACT TYPE default::FolderLike EXTENDING default::TreeElement;
  CREATE ABSTRACT TYPE default::VisibleTreeElement EXTENDING default::TreeElement {
      CREATE REQUIRED LINK parent -> default::FolderLike {
          ON TARGET DELETE  DELETE SOURCE;
      };
      CREATE PROPERTY deleted_at -> std::datetime;
      CREATE REQUIRED PROPERTY name -> std::str;
      CREATE CONSTRAINT std::exclusive ON ((.project, .parent, .name));
  };
  CREATE ABSTRACT TYPE default::ContentElement EXTENDING default::VisibleTreeElement {
      CREATE REQUIRED PROPERTY size -> std::int64;
  };
  CREATE TYPE default::Doc EXTENDING default::ContentElement {
      CREATE PROPERTY in_s3 -> std::bool;
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
      CREATE LINK rootDoc -> default::Doc {
          ON TARGET DELETE  ALLOW;
      };
  };
  CREATE ABSTRACT TYPE default::OneTimeToken {
      CREATE REQUIRED PROPERTY created_at -> std::datetime;
      CREATE REQUIRED PROPERTY expires_at -> std::datetime;
      CREATE REQUIRED PROPERTY token -> std::str {
          CREATE CONSTRAINT std::exclusive;
      };
      CREATE REQUIRED PROPERTY used_at -> std::datetime;
  };
  CREATE TYPE default::EmailConfirmationToken EXTENDING default::OneTimeToken {
      CREATE REQUIRED LINK email -> default::Email {
          ON TARGET DELETE  DELETE SOURCE;
      };
  };
  CREATE TYPE default::PasswordResetToken EXTENDING default::OneTimeToken {
      CREATE REQUIRED LINK email -> default::Email {
          ON TARGET DELETE  DELETE SOURCE;
      };
  };
  CREATE TYPE default::User {
      CREATE REQUIRED LINK email -> default::Email;
      CREATE REQUIRED MULTI LINK emails -> default::Email {
          CREATE CONSTRAINT std::exclusive;
      };
      CREATE REQUIRED LINK features -> default::Features;
      CREATE MULTI LINK contacts -> default::User {
          ON TARGET DELETE  DELETE SOURCE;
          CREATE PROPERTY connections -> std::int64;
          CREATE PROPERTY last_touched -> std::datetime;
      };
      CREATE REQUIRED PROPERTY editor_config -> std::json {
          SET default := (<std::json>'{"autoComplete":true,"autoPairDelimiters":true,"fontFamily":"lucida","fontSize":12,"lineHeight":"normal","mode":"default","overallTheme":"","pdfViewer":"pdfjs","syntaxValidation":false,"spellCheckLanguage":"en","theme":"textmate"}');
      };
      CREATE REQUIRED PROPERTY epoch -> std::int64 {
          SET default := 1;
      };
      CREATE PROPERTY first_name -> std::str;
      CREATE PROPERTY last_logged_in -> std::datetime;
      CREATE PROPERTY last_login_ip -> std::str;
      CREATE PROPERTY last_name -> std::str;
      CREATE MULTI PROPERTY learned_words -> std::str;
      CREATE REQUIRED PROPERTY login_count -> std::int64 {
          SET default := 0;
      };
      CREATE PROPERTY must_reconfirm -> std::bool;
      CREATE REQUIRED PROPERTY password_hash -> std::str;
      CREATE REQUIRED PROPERTY signup_date -> std::datetime {
          SET default := (std::datetime_of_transaction());
      };
  };
  CREATE TYPE default::Folder EXTENDING default::VisibleTreeElement, default::FolderLike;
  CREATE TYPE default::RootFolder EXTENDING default::FolderLike {
      CREATE CONSTRAINT std::exclusive ON (.project);
  };
  CREATE TYPE default::Message {
      CREATE REQUIRED LINK room -> default::Room {
          ON TARGET DELETE  DELETE SOURCE;
      };
      CREATE REQUIRED LINK user -> default::User;
      CREATE REQUIRED PROPERTY content -> std::str;
      CREATE REQUIRED PROPERTY created_at -> std::datetime {
          SET default := (std::datetime_of_transaction());
      };
      CREATE PROPERTY edited_at -> std::datetime;
  };
  CREATE TYPE default::Notification {
      CREATE REQUIRED LINK user -> default::User {
          ON TARGET DELETE  DELETE SOURCE;
      };
      CREATE REQUIRED PROPERTY expires_at -> std::datetime;
      CREATE REQUIRED PROPERTY key -> std::str;
      CREATE PROPERTY message_options -> std::json;
      CREATE PROPERTY template_key -> std::str;
  };
  ALTER TYPE default::Project {
      CREATE MULTI LINK access_ro -> default::User {
          ON TARGET DELETE  ALLOW;
      };
      CREATE MULTI LINK access_rw -> default::User {
          ON TARGET DELETE  ALLOW;
      };
      CREATE MULTI LINK archived_by -> default::User {
          ON TARGET DELETE  ALLOW;
      };
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
      CREATE LINK last_updated_by -> default::User {
          ON TARGET DELETE  ALLOW;
      };
      CREATE REQUIRED LINK owner -> default::User {
          ON TARGET DELETE  DELETE SOURCE;
      };
      CREATE MULTI LINK token_access_ro -> default::User {
          ON TARGET DELETE  ALLOW;
      };
      CREATE MULTI LINK token_access_rw -> default::User {
          ON TARGET DELETE  ALLOW;
      };
      CREATE MULTI LINK trashed -> default::User {
          ON TARGET DELETE  ALLOW;
      };
  };
  CREATE TYPE default::ProjectInvite {
      CREATE REQUIRED LINK project -> default::Project {
          ON TARGET DELETE  DELETE SOURCE;
      };
      CREATE REQUIRED LINK sending_user -> default::User {
          ON TARGET DELETE  DELETE SOURCE;
      };
      CREATE REQUIRED PROPERTY created_at -> std::datetime {
          SET default := (std::datetime_of_transaction());
      };
      CREATE REQUIRED PROPERTY email -> std::str;
      CREATE REQUIRED PROPERTY expires_at -> std::datetime;
      CREATE REQUIRED PROPERTY privilege_level -> std::str;
      CREATE REQUIRED PROPERTY token -> std::str;
  };
  CREATE TYPE default::ReviewThread EXTENDING default::Room {
      CREATE LINK resolved_by -> default::User;
      CREATE PROPERTY resolved_at -> std::datetime;
  };
  CREATE TYPE default::Tag {
      CREATE MULTI LINK projects -> default::Project {
          ON TARGET DELETE  ALLOW;
      };
      CREATE REQUIRED LINK user -> default::User {
          ON TARGET DELETE  DELETE SOURCE;
      };
      CREATE REQUIRED PROPERTY name -> std::str;
  };
  CREATE TYPE default::SystemMessage {
      CREATE REQUIRED PROPERTY content -> std::str;
  };
  CREATE TYPE default::UserAuditLogEntry {
      CREATE REQUIRED LINK initiator -> default::User;
      CREATE PROPERTY info -> std::json;
      CREATE REQUIRED PROPERTY ip_address -> std::str;
      CREATE REQUIRED PROPERTY operation -> std::str;
      CREATE REQUIRED PROPERTY timestamp -> std::datetime;
  };
  ALTER TYPE default::User {
      CREATE MULTI LINK audit_log -> default::UserAuditLogEntry {
          ON TARGET DELETE  ALLOW;
      };
  };
};
