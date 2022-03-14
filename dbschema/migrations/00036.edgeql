CREATE MIGRATION m1im67b3dwdjih4pbwslvayyl25sdsczc56trzocmmzj42zeqcrf7a
    ONTO m15xobmucnayblcxzpohfvz4arqbdjls5ml6iwggk4ii2jomnaku2q
{
  ALTER TYPE default::Project {
      CREATE REQUIRED PROPERTY track_changes -> std::json {
          SET REQUIRED USING (SELECT
              <std::json>'{}'
          );
      };
  };
};
