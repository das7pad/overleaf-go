CREATE MIGRATION m123ljj5tfgsnz647r3dxvf2bfnub7wxhqovjhhwe6onsnkxjbqmsa
    ONTO m1msjkrg6k5jfgsjudl7u3tj3npjjjkydfq5i5xqpt2wxhiigi2kdq
{
  DROP TYPE default::EmailConfirmationToken;
  ALTER TYPE default::OneTimeToken {
      CREATE REQUIRED PROPERTY use -> std::str {
          SET REQUIRED USING (SELECT
              {'password'}
          );
      };
  };
  DROP TYPE default::PasswordResetToken;
};
