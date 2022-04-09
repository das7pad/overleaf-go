CREATE MIGRATION m1lzo6rdilxhtitxxrx5nvezvu2torphkg7kpsmdeqkgatq4exzgoa
    ONTO m1osu3te4fpztyvzqhocmycsc2udm27fl2aqkboo4xw4xs7kc2ja6q
{
  ALTER TYPE default::LinkedFileData {
      CREATE REQUIRED LINK file -> default::File {
          ON TARGET DELETE  DELETE SOURCE;
          SET REQUIRED USING (SELECT
              default::File FILTER
                  (.linked_file_data = default::LinkedFileData)
          LIMIT
              1
          );
      };
  };
};
