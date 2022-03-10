CREATE MIGRATION m1n6epxpxmmbo4mwlmrgngwdbrqxkd2cz4kz2dlpbxfe7gr6nkk7la
    ONTO m1z737mgdfhfiw4jnhc7wetqplhfo72bhsbpkavqxymu5cmkb7fzya
{
  ALTER TYPE default::User {
      ALTER LINK email {
          DROP CONSTRAINT std::exclusive;
      };
  };
  ALTER TYPE default::User {
      ALTER LINK email {
          USING (SELECT
              default::Email FILTER
                  (.user = default::User)
              ORDER BY
                  .preference ASC
          LIMIT
              1
          );
          RESET OPTIONALITY;
      };
      ALTER LINK emails {
          DROP CONSTRAINT std::exclusive;
      };
  };
  ALTER TYPE default::User {
      ALTER LINK emails {
          USING (.<user[IS default::Email]);
          RESET OPTIONALITY;
      };
  };
};
