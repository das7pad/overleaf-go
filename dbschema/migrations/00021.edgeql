CREATE MIGRATION m177ftb2vs3pq5mdm3h5776pdvphqlnpruu2z6l2hrpvhaezyvudgq
    ONTO m1setqfjw335jin2zezhilsa5e3xdvr5mi5cyhwdzpofetikg74ema
{
  ALTER TYPE default::User {
      ALTER LINK emails {
          RESET EXPRESSION;
          RESET EXPRESSION;
          SET REQUIRED USING (SELECT
              default::Email
          FILTER
              (.user = default::User)
          );
          SET TYPE default::Email;
      };
  };
  ALTER TYPE default::User {
      ALTER LINK emails {
          SET REQUIRED USING (SELECT
              default::Email
          FILTER
              (.user = default::User)
          );
      };
  };
};
