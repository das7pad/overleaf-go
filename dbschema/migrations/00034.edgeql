CREATE MIGRATION m1eogboap3gac3u36cmluqcaravfhzqkzbusurlfdhfww3jr2tfpla
    ONTO m1v5yjtc4sxq4rohh53u5ks2m65mm5tazbacjuidmsmk2llfkzuhzq
{
  ALTER TYPE default::Project {
      ALTER LINK members {
          SET MULTI;
      };
  };
  ALTER TYPE default::User {
      ALTER LINK projects_owned {
          SET MULTI;
      };
  };
  ALTER TYPE default::User {
      ALTER LINK projects_ro {
          SET MULTI;
      };
  };
  ALTER TYPE default::User {
      ALTER LINK projects_rw {
          SET MULTI;
      };
  };
  ALTER TYPE default::User {
      ALTER LINK projects_token_ro {
          SET MULTI;
      };
  };
  ALTER TYPE default::User {
      ALTER LINK projects_token_rw {
          SET MULTI;
      };
  };
  ALTER TYPE default::User {
      ALTER LINK projects {
          SET MULTI;
      };
  };
  ALTER TYPE default::User {
      ALTER LINK tags {
          SET MULTI;
      };
  };
};
