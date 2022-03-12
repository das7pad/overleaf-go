CREATE MIGRATION m1ozrfhyjr75najc7zhp7wfqk54wzpyniejc4shj4lxhgose3rs2hq
    ONTO m1qw7tjxgtlkuzufnwgrr37hstzqxz4kpz5ozyyzq7xc5664gf5prq
{
  ALTER TYPE default::Email {
      DROP CONSTRAINT std::exclusive ON ((.user, .preference));
      DROP LINK user;
      DROP PROPERTY preference;
  };
  ALTER TYPE default::User {
      ALTER LINK email {
          CREATE CONSTRAINT std::exclusive;
      };
      ALTER LINK emails {
          CREATE CONSTRAINT std::exclusive;
      };
  };
};
