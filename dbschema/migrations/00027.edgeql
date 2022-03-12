CREATE MIGRATION m1bzyjbrzhab3a5azcpdw5sk47q5klkt36xpdkheqnu6ycaucwkbrq
    ONTO m1775ljeufl7euiruxa7pubfjng4jowszhprymvmsmgbtwhgmqzvga
{
  ALTER TYPE default::User {
      CREATE LINK projects_owned := (.<owner[IS default::Project]);
      CREATE LINK projects_ro := (.<access_ro[IS default::Project]);
      CREATE LINK projects_rw := (.<access_rw[IS default::Project]);
      CREATE LINK projects_token_ro := (SELECT
          default::Project
      FILTER
          ((default::User IN .token_access_ro) AND (.public_access_level = 'tokenBased'))
      );
      CREATE LINK projects_token_rw := (SELECT
          default::Project
      FILTER
          ((default::User IN .token_access_rw) AND (.public_access_level = 'tokenBased'))
      );
  };
};
