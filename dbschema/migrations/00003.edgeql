CREATE MIGRATION m1qmhn5iyn2vud5ej5376smly65bd26nkvw4xigkzejosgs7lx4nqq
    ONTO m1elontv6ocznarlr5h26p7ti2pt37om473gdkmb3i73en6aktdl3a
{
  ALTER TYPE default::LinkedFileData {
      ALTER PROPERTY source_entity_path {
          SET REQUIRED USING (SELECT
              ''
          );
      };
  };
  ALTER TYPE default::LinkedFileData {
      ALTER PROPERTY source_output_file_path {
          SET REQUIRED USING (SELECT
              ''
          );
      };
  };
  ALTER TYPE default::LinkedFileData {
      ALTER PROPERTY source_project_id {
          SET REQUIRED USING (SELECT
              ''
          );
      };
  };
  ALTER TYPE default::LinkedFileData {
      ALTER PROPERTY url {
          SET REQUIRED USING (SELECT
              ''
          );
      };
  };
};
