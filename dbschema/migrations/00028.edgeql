CREATE MIGRATION m12nciqya3obgv37lyoq4m4tmb7dxrm4rj4lhkkru67hlo2cgxdzyq
    ONTO m1bzyjbrzhab3a5azcpdw5sk47q5klkt36xpdkheqnu6ycaucwkbrq
{
  ALTER TYPE default::Project {
      ALTER LINK last_updated_by {
          SET REQUIRED USING (SELECT
              .owner
          );
      };
  };
  ALTER TYPE default::Project {
      ALTER PROPERTY last_updated_at {
          SET REQUIRED USING (SELECT
              {std::datetime_of_transaction()}
          );
      };
  };
};
