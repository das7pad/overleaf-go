CREATE MIGRATION m1ypqynpsmlav2qlxd5rkutfirgskhuwjrvvuhedxu464j2hguk4qa
    ONTO m1db2yqosnay526oifb76hwsgn2qpucgpwnlsu3cwy4kc3gq32vyha
{
  ALTER TYPE default::Room {
      CREATE MULTI LINK messages := (.<room[IS default::Message]);
  };
  ALTER TYPE default::Project {
      CREATE MULTI LINK docs := (.<project[IS default::Doc]);
      CREATE MULTI LINK files := (.<project[IS default::File]);
      CREATE MULTI LINK folders := (.<project[IS default::Folder]);
      CREATE LINK root_folder := (.<project[IS default::RootFolder]);
  };
};
