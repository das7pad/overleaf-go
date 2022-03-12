CREATE MIGRATION m1xsychlm662yywrsqr4rrzl4oztb6ctmbwt7ldjzdkr3dp2llon4a
    ONTO m1qyii5b6bkcufatu7zns7i3zk6bty4z4dfpys5e73n3ymqy565qta
{
  ALTER TYPE default::Doc {
      ALTER PROPERTY in_s3 {
          SET REQUIRED USING (SELECT
              {false}
          );
      };
  };
  ALTER TYPE default::Notification {
      ALTER PROPERTY message_options {
          SET REQUIRED USING (SELECT
              {<std::json>'{}'}
          );
      };
  };
  ALTER TYPE default::Notification {
      ALTER PROPERTY template_key {
          SET REQUIRED USING (SELECT
              {''}
          );
      };
  };
  ALTER TYPE default::User {
      ALTER PROPERTY first_name {
          SET REQUIRED USING (SELECT
              {''}
          );
      };
  };
  ALTER TYPE default::User {
      ALTER PROPERTY last_name {
          SET REQUIRED USING (SELECT
              {''}
          );
      };
  };
  ALTER TYPE default::User {
      ALTER PROPERTY must_reconfirm {
          SET REQUIRED USING (SELECT
              {false}
          );
      };
  };
};
