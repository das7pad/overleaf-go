CREATE MIGRATION m1p3hfsveourfscs4xi7axl2jxwxsl5sklbfqv2tdnggjqwdnbod5q
    ONTO m1lasydn7f7dddkjxdgeh7mslyz3rnkvysj7npw7qcbjmwru6ie5hq
{
  ALTER TYPE default::Email {
      CREATE REQUIRED LINK user -> default::User {
          ON TARGET DELETE  DELETE SOURCE;
          SET REQUIRED USING (SELECT
              default::User 
          LIMIT
              1
          );
      };
      CREATE REQUIRED PROPERTY preference -> std::int64 {
          SET REQUIRED USING (SELECT
              1
          );
      };
      CREATE CONSTRAINT std::exclusive ON ((.user, .preference));
  };
};
