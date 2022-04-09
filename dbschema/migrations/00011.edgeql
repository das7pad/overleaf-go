CREATE MIGRATION m15s5cjgoryneln4i26lqrtpoa4k6j26o7yagu64sotw5gzxckgv3a
    ONTO m1biug55fjsoaklcygrfkrzhbdlqbiwzkfla3zaqgze7fdwtdmdrma
{
  ALTER TYPE default::ProjectAuditLogEntry {
      CREATE REQUIRED LINK project -> default::Project {
          ON TARGET DELETE  DELETE SOURCE;
          SET REQUIRED USING (SELECT
              default::Project FILTER
                  (default::ProjectAuditLogEntry IN .audit_log)
          LIMIT
              1
          );
      };
  };
  ALTER TYPE default::UserAuditLogEntry {
      CREATE REQUIRED LINK user -> default::User {
          ON TARGET DELETE  DELETE SOURCE;
          SET REQUIRED USING (SELECT
              default::User FILTER
                  (default::UserAuditLogEntry IN .audit_log)
          LIMIT
              1
          );
      };
  };
};
