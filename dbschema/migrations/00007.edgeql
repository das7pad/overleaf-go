CREATE MIGRATION m1osu3te4fpztyvzqhocmycsc2udm27fl2aqkboo4xw4xs7kc2ja6q
    ONTO m1ypknroljbfg75ccyilrvcbrh3uemcuooyuyi2vnqsaznfpeygbpq
{
  ALTER TYPE default::Project {
      CREATE MULTI LINK all_files := (.<project[IS default::File]);
      ALTER LINK root_doc {
          RESET ON TARGET DELETE;
      };
  };
};
