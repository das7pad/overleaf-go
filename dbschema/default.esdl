module default {
  type Email {
    required property email -> str {
      constraint exclusive;
    }
    required property created_at -> datetime {
      default := datetime_of_transaction();
    }
    property confirmed_at -> datetime;
    link user := .<emails[is User];
  }

  type Features {
    required property compile_group -> str;
    required property compile_timeout -> duration;
  }

  type User {
    multi link audit_log -> UserAuditLogEntry {
      on target delete allow;
    }
    multi link contacts -> User {
      on target delete delete source;
      property connections -> int64;
      property last_touched -> datetime;
    }
    required property editor_config -> json {
      default := to_json('{"autoComplete":true,"autoPairDelimiters":true,"fontFamily":"lucida","fontSize":12,"lineHeight":"normal","mode":"default","overallTheme":"","pdfViewer":"pdfjs","syntaxValidation":false,"spellCheckLanguage":"en","theme":"textmate"}');
    }
    required link email -> Email {
      constraint exclusive;
    }
    required multi link emails -> Email {
      constraint exclusive;
    }
    required property epoch -> int64 {
      default := 1;
    }
    required link features -> Features;
    required property first_name -> str;
    required property last_name -> str;
    required property password_hash -> str;
    property last_logged_in -> datetime;
    property last_login_ip -> str;
    required property login_count -> int64 {
      default := 0;
    }
    required property must_reconfirm -> bool {
      default := false;
    }
    required property signup_date -> datetime {
      default := datetime_of_transaction();
    }
    multi property learned_words -> str;

    multi link projects := distinct (
      .projects_owned
      union .projects_ro
      union .projects_rw
      union .projects_token_ro
      union .projects_token_rw
    );
    multi link projects_owned := .<owner[is Project];
    multi link projects_ro := .<access_ro[is Project];
    multi link projects_rw := .<access_rw[is Project];
    multi link projects_token_rw := (
      select Project
      filter User in .access_token_rw and .public_access_level = 'tokenBased'
    );
    multi link projects_token_ro := (
      select Project
      filter User in .access_token_ro and .public_access_level = 'tokenBased'
    );
    multi link tags := .<user[is Tag];
  }

  type UserAuditLogEntry {
    required link initiator -> User;
    required property timestamp -> datetime {
      default := datetime_of_transaction();
    }
    required property operation -> str;
    required property ip_address -> str;
    property info -> json;
  }

  type ProjectAuditLogEntry {
    required link initiator -> User;
    required property timestamp -> datetime {
      default := datetime_of_transaction();
    }
    required property operation -> str;
    property info -> json;
  }

  type Tokens {
    # Keep detached from Project to ensure no re-use of tokens after the
    #  deletion of a project.
    required property token_ro -> str {
      constraint exclusive;
    }
    required property token_prefix_rw -> str {
      constraint exclusive;
    }
    required property token_rw -> str {
      constraint exclusive;
    }
  }

  type Project {
    required property active -> bool {
      default := true;
    }
    required property version -> int64 {
      default := 1;
    }
    multi link audit_log -> ProjectAuditLogEntry {
      on target delete allow;
    }
    multi link archived_by -> User {
      on target delete allow;
    }
    multi link access_rw -> User {
      on target delete allow;
    }
    required property compiler -> str;
    required property epoch -> int64 {
      default := 1;
    }
    required property image_name -> str;
    property last_opened -> datetime;
    required property last_updated_at -> datetime {
      default := datetime_of_transaction();
    }
    required link last_updated_by -> User {
      on target delete allow;
    }
    required property name -> str;
    required link owner -> User {
      on target delete delete source;
    }
    required property public_access_level -> str {
      default := 'private';
    }
    multi link access_ro -> User {
      on target delete allow;
    }
    link root_doc -> Doc {
      on target delete allow;
    }
    required property spell_check_language -> str;
    multi link access_token_rw -> User {
      on target delete allow;
    }
    multi link access_token_ro -> User {
      on target delete allow;
    }
    link tokens -> Tokens {
      constraint exclusive;
    }
    multi link trashed_by -> User {
      on target delete allow;
    }

    multi link members := distinct (.access_ro union .access_rw);

    link root_folder := .<project[is RootFolder];
    multi link any_folders := .<project[is FolderLike];
    multi link folders := .<project[is Folder];
    multi link docs := .<project[is Doc];
    multi link files := .<project[is File];
  }

  abstract type TreeElement {
    required link project -> Project {
      on target delete delete source;
    }
  }

  abstract type FolderLike extending TreeElement {
      multi link folders := .<parent[is Folder];
      multi link docs := .<parent[is Doc];
      multi link files := .<parent[is File];
  }

  type RootFolder extending FolderLike {
    constraint exclusive on ((.project));
  }

  abstract type VisibleTreeElement extending TreeElement {
    required link parent -> FolderLike {
      on target delete delete source;
    }
    required property name -> str;
    property deleted_at -> datetime;
    constraint exclusive on ((.project, .parent, .name));
  }

  type Folder extending VisibleTreeElement, FolderLike {
  }

  abstract type ContentElement extending VisibleTreeElement {
    required property size -> int64;
  }

  type Doc extending ContentElement {
    required property snapshot -> str;
    required property version -> int64;
    required property in_s3 -> bool {
      default := false;
    }
  }

  type File extending ContentElement {
    required property created_at -> datetime {
      default := datetime_of_transaction();
    }
    required property hash -> str;
  }

  type LinkedFileURL extending File {
    required property url -> str;
  }

  type LinkedFileProjectFile extending File {
    link source_element -> ContentElement {
      on target delete allow;
    }
  }

  type LinkedFileProjectOutputFile extending File {
    link source_project -> Project {
      on target delete allow;
    }
    required property source_path -> str;
  }

  type ProjectInvite {
    required property created_at -> datetime {
      default := datetime_of_transaction();
    }
    required property email -> str;
    required property expires_at -> datetime;
    required property privilege_level -> str;
    required link project -> Project {
      on target delete delete source;
    }
    required link sending_user -> User {
      on target delete delete source;
    }
    required property token -> str;
  }

  type OneTimeToken {
    required property token -> str {
      constraint exclusive;
    }
    required property created_at -> datetime {
      default := datetime_of_transaction();
    }
    required property expires_at -> datetime;
    property used_at -> datetime;
    required link email -> Email {
      on target delete delete source;
    }
    required property use -> str;
  }

  type Notification {
    required property key -> str;
    required property expires_at -> datetime;
    required link user -> User {
      on target delete delete source;
    }
    required property template_key -> str;
    required property message_options -> json;
    constraint exclusive on ((.key, .user));
  }

  type Tag {
    required link user -> User {
      on target delete delete source;
    }
    required property name -> str;
    multi link projects -> Project {
      on target delete allow;
    }
    constraint exclusive on ((.name, .user));
  }

  type SystemMessage {
    required property content -> str;
  }

  abstract type Room {
    required link project -> Project {
      on target delete delete source;
    }
    multi link messages := .<room[is Message];
  }

  type ChatRoom extending Room {
    constraint exclusive on ((.project));
  }

  type ReviewThread extending Room {
    property resolved_at -> datetime;
    link resolved_by -> User;
  }

  type Message {
    required property content -> str;
    required property created_at -> datetime {
      default := datetime_of_transaction();
    }
    required link user -> User;
    required link room -> Room {
      on target delete delete source;
    }
    property edited_at -> datetime;
  }
}
