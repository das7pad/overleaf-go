CREATE MIGRATION m1elontv6ocznarlr5h26p7ti2pt37om473gdkmb3i73en6aktdl3a
    ONTO m1bv5sb33s2wzs233ixyniobpzuda2oc5gn3s6prg2kalw5ry7pj5a
{
  CREATE TYPE default::LinkedFileData {
      CREATE REQUIRED PROPERTY provider -> std::str;
      CREATE PROPERTY source_entity_path -> std::str;
      CREATE PROPERTY source_output_file_path -> std::str;
      CREATE PROPERTY source_project_id -> std::str;
      CREATE PROPERTY url -> std::str;
  };
  ALTER TYPE default::File {
      CREATE LINK linked_file_data -> default::LinkedFileData;
  };
  DROP TYPE default::LinkedFileProjectFile;
  DROP TYPE default::LinkedFileProjectOutputFile;
  DROP TYPE default::LinkedFileURL;
};
