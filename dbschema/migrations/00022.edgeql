CREATE MIGRATION m1qw7tjxgtlkuzufnwgrr37hstzqxz4kpz5ozyyzq7xc5664gf5prq
    ONTO m177ftb2vs3pq5mdm3h5776pdvphqlnpruu2z6l2hrpvhaezyvudgq
{
  ALTER TYPE default::User {
      ALTER LINK email {
          RESET EXPRESSION;
          RESET EXPRESSION;
          RESET CARDINALITY;
          SET REQUIRED USING (SELECT
              default::Email FILTER
                  (.user = default::User)
              ORDER BY
                  .preference ASC
          LIMIT
              1
          );
          SET TYPE default::Email;
      };
  };
  ALTER TYPE default::User {
      ALTER LINK email {
          SET REQUIRED USING (SELECT
              default::Email FILTER
                  (.user = default::User)
              ORDER BY
                  .preference ASC
          LIMIT
              1
          );
      };
  };
};
