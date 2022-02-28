CREATE MIGRATION m1dw3fdsra3d5exzyickmwbruicd3yty2gs5u3msnn6zvr3ewjyh2q
    ONTO m14lsnu76bwfrbpls75vgsildg4hqdp6rsmdxi6bc7wvjoq2f5r2cq
{
  ALTER TYPE default::Project {
      ALTER LINK tokens {
          RESET OPTIONALITY;
      };
  };
  ALTER TYPE default::Project {
      CREATE REQUIRED PROPERTY active -> std::bool {
          SET default := true;
      };
  };
  ALTER TYPE default::Project {
      ALTER PROPERTY last_updated_at {
          SET default := (std::datetime_of_transaction());
      };
  };
  ALTER TYPE default::Project {
      ALTER PROPERTY public_access_level {
          SET default := 'private';
          SET REQUIRED USING (SELECT
              'private'
          );
      };
  };
};
