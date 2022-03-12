CREATE MIGRATION m1775ljeufl7euiruxa7pubfjng4jowszhprymvmsmgbtwhgmqzvga
    ONTO m1xsychlm662yywrsqr4rrzl4oztb6ctmbwt7ldjzdkr3dp2llon4a
{
  ALTER TYPE default::Doc {
      ALTER PROPERTY in_s3 {
          SET default := false;
      };
  };
  ALTER TYPE default::User {
      ALTER PROPERTY must_reconfirm {
          SET default := false;
      };
  };
};
