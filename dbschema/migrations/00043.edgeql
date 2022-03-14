CREATE MIGRATION m1bnsddckkxb2l4hlop3rinqial3x2iqeljzecutfqfqw2lmil3dhq
    ONTO m1j6g6esrys3ehkkl4zjkd665y3suucfud6tnnwiwju6ywhegyhjiq
{
  ALTER TYPE default::Project {
      ALTER LINK root_folder {
          USING (SELECT
              .<project[IS default::RootFolder] 
          LIMIT
              1
          );
      };
  };
};
