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

  type EditorConfig {
    required property auto_complete -> bool;
    required property auto_pair_delimiters -> bool;
    required property font_family -> str;
    required property font_size -> int64;
    required property line_height -> str;
    required property mode -> str;
    required property overall_theme -> str;
    required property pdf_viewer -> str;
    required property syntax_validation -> bool;
    required property spell_check_language -> str;
    required property theme -> str;
  }

  type User {
    property deleted_at -> datetime;
    multi link audit_log := .<user[is UserAuditLogEntry];
    required property beta_program -> bool {
      default := false;
    }
    multi link contacts -> User {
      on target delete allow;
      property connections -> int64;
      property last_touched -> datetime;
    }
    required link editor_config -> EditorConfig;
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
    multi link projects := distinct (
      .projects_owned
      union .projects_ro
      union .projects_rw
      union .projects_token_ro
      union .projects_token_rw
    );
    multi link tags := .<user[is Tag];
  }

  type UserAuditLogEntry {
    required link user -> User {
      on target delete delete source;
    }
    link initiator -> User {
      on target delete allow;
    }
    required property timestamp -> datetime {
      default := datetime_of_transaction();
    }
    required property operation -> str;
    required property ip_address -> str;
    property info -> json;
  }

  type ProjectAuditLogEntry {
    required link project -> Project {
      on target delete delete source;
    }
    link initiator -> User {
      on target delete allow;
    }
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
    property deleted_at -> datetime;
    required property version -> int64 {
      default := 1;
    }
    multi link audit_log := .<project[is ProjectAuditLogEntry];
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
    required property track_changes_state -> json {
      default := to_json('{}');
    }
    required link owner -> User;
    required property public_access_level -> str {
      default := 'private';
    }
    multi link access_ro -> User {
      on target delete allow;
    }
    link root_doc -> Doc;
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

    multi link invites := .<project[is ProjectInvite];

    multi link min_access_ro := distinct (
      {.owner}
      union .access_ro union .access_rw
      union .access_token_ro union .access_token_rw
    );
    multi link min_access_rw := distinct (
      {.owner} union .access_rw union .access_token_rw
    );

    link chat := (select .<project[is ChatRoom] limit 1);
    link root_folder := (
      select .<project[is RootFolder] limit 1
    );
    multi link folders := (
      select .<project[is Folder] filter not .deleted
    );
    multi link docs := (
      select .<project[is Doc] filter not .deleted
    );
    multi link files := (
      select .<project[is File] filter not .deleted
    );
    multi link deleted_docs := (
      select .<project[is Doc] filter .deleted
    );

    multi link all_files := .<project[is File];
  }

  abstract type TreeElement {
    required link project -> Project {
      on target delete delete source;
    }
  }

  abstract type FolderLike extending TreeElement {
    required property path -> str;
    property path_for_join := (
      '' if .path = '' else (.path ++ '/')
    );

    multi link folders := (
      select .<parent[is Folder] filter not .deleted
    );
    multi link docs := (
      select .<parent[is Doc] filter not .deleted
    );
    multi link files := (
      select .<parent[is File] filter not .deleted
    );
  }

  type RootFolder extending FolderLike {
    constraint exclusive on ((.project));
  }

  abstract type VisibleTreeElement extending TreeElement {
    required link parent -> FolderLike {
      on target delete delete source;
    }
    required property name -> str;
    property deleted := .deleted_at != <datetime>'1970-01-01T00:00:00.000000Z';
    required property deleted_at -> datetime {
      default := <datetime>'1970-01-01T00:00:00.000000Z';
    }
    constraint exclusive on ((.project, .parent, .name, .deleted_at));
  }

  type Folder extending VisibleTreeElement, FolderLike {
  }

  abstract type ContentElement extending VisibleTreeElement {
    required property size -> int64;
    property resolved_path := .parent.path_for_join ++ .name;
  }

  type Doc extending ContentElement {
    required property snapshot -> str;
    required property version -> int64;
  }

  type LinkedFileData {
    required property provider -> str;
    required property source_project_id -> str;
    required property source_entity_path -> str;
    required property source_output_file_path -> str;
    required property url -> str;

    required link file -> File {
      on target delete delete source;
    }
  }

  type File extending ContentElement {
    required property created_at -> datetime {
      default := datetime_of_transaction();
    }
    required property hash -> str;
    link linked_file_data := (
      select .<file[is LinkedFileData] limit 1
    );
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

    constraint exclusive on ((.project, .token));
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

  type ProjectInviteNotification extending Notification {
    required link project_invite -> ProjectInvite {
      on target delete delete source;
    }
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
