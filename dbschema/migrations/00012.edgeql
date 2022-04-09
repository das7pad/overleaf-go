CREATE MIGRATION m1qnddlsbajpnboiy7pzk5tzxg4mry4iiledqhzwxjgsh5q6b56mwa
    ONTO m15s5cjgoryneln4i26lqrtpoa4k6j26o7yagu64sotw5gzxckgv3a
{
  ALTER TYPE default::Project {
      ALTER LINK audit_log {
          USING (.<project[IS default::ProjectAuditLogEntry]);
          RESET ON TARGET DELETE;
      };
  };
  ALTER TYPE default::User {
      ALTER LINK audit_log {
          USING (.<user[IS default::UserAuditLogEntry]);
          RESET ON TARGET DELETE;
      };
  };
};
