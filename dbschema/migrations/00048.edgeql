CREATE MIGRATION m1sw5ssehspylr5z7cncgncjgbs4ukuqml4ttfn5qybdgyr25ef3ba
    ONTO m1o77tbdl7tfcdnvuudwq4jqjfn2c7tgqu34zytnbkgykpztzq3oya
{
  ALTER TYPE default::User {
      CREATE REQUIRED PROPERTY beta_program -> std::bool {
          SET default := false;
      };
  };
};
