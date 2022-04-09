CREATE MIGRATION m1ypknroljbfg75ccyilrvcbrh3uemcuooyuyi2vnqsaznfpeygbpq
    ONTO m1m2igb575cksepg3myqlf2ybwnvtwwzaqipwygdq6x2gl247hekaq
{
  ALTER TYPE default::Project {
      ALTER LINK owner {
          RESET ON TARGET DELETE;
      };
  };
  ALTER TYPE default::ProjectAuditLogEntry {
      ALTER LINK initiator {
          ON TARGET DELETE  ALLOW;
          RESET OPTIONALITY;
      };
  };
  CREATE TYPE default::ProjectInviteNotification EXTENDING default::Notification {
      CREATE REQUIRED LINK project_invite -> default::ProjectInvite {
          ON TARGET DELETE  DELETE SOURCE;
      };
  };
  DROP TYPE default::ReviewThread;
  ALTER TYPE default::User {
      ALTER LINK contacts {
          ON TARGET DELETE  ALLOW;
      };
  };
  ALTER TYPE default::UserAuditLogEntry {
      ALTER LINK initiator {
          ON TARGET DELETE  ALLOW;
          RESET OPTIONALITY;
      };
  };
};
