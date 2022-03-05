CREATE MIGRATION m1few24crlv7sd6guattj7gq7d5wna7w6kogscxmalp4a5vnvewv5q
    ONTO m1af6npikafibhmwiogq6dsesv6usyxqzu3n4ozsgjq6qgpzt54r5q
{
  ALTER TYPE default::Project {
      CREATE MULTI LINK any_folders := (.<project[IS default::FolderLike]);
  };
};
