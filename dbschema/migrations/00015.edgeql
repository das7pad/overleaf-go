CREATE MIGRATION m1msjkrg6k5jfgsjudl7u3tj3npjjjkydfq5i5xqpt2wxhiigi2kdq
    ONTO m1n6epxpxmmbo4mwlmrgngwdbrqxkd2cz4kz2dlpbxfe7gr6nkk7la
{
  ALTER TYPE default::OneTimeToken {
      CREATE REQUIRED LINK email -> default::Email {
          ON TARGET DELETE  DELETE SOURCE;
          SET REQUIRED USING (SELECT
              default::Email 
          LIMIT
              1
          );
      };
  };
  ALTER TYPE default::EmailConfirmationToken {
      ALTER LINK email {
          RESET OPTIONALITY;
          DROP OWNED;
          RESET TYPE;
      };
  };
  ALTER TYPE default::PasswordResetToken {
      ALTER LINK email {
          RESET OPTIONALITY;
          DROP OWNED;
          RESET TYPE;
      };
  };
};
