CREATE MIGRATION m14lsnu76bwfrbpls75vgsildg4hqdp6rsmdxi6bc7wvjoq2f5r2cq
    ONTO m1pizgkosxeyp4amf7qlviihcgr7cilvmubd7p7tnfhnp3ijbvfnsq
{
  ALTER TYPE default::ChatRoom {
      CREATE CONSTRAINT std::exclusive ON (.project);
  };
  ALTER TYPE default::Project {
      ALTER LINK tokens {
          CREATE CONSTRAINT std::exclusive;
      };
  };
  ALTER TYPE default::Tag {
      CREATE CONSTRAINT std::exclusive ON ((.name, .user));
  };
  ALTER TYPE default::User {
      ALTER LINK email {
          CREATE CONSTRAINT std::exclusive;
      };
  };
};
