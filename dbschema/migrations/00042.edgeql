CREATE MIGRATION m1j6g6esrys3ehkkl4zjkd665y3suucfud6tnnwiwju6ywhegyhjiq
    ONTO m136ut7idfaboyjedrpn7nlwvsponwq3nc7oubog37qasazckpfowa
{
  ALTER TYPE default::User {
      ALTER LINK editor_config {
          SET REQUIRED USING (SELECT
              default::EditorConfig
          FILTER
              (.id = <std::uuid>'cec0da5c-a3a2-11ec-bef6-377970381513')
          );
      };
  };
};
