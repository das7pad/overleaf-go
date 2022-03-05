CREATE MIGRATION m1af6npikafibhmwiogq6dsesv6usyxqzu3n4ozsgjq6qgpzt54r5q
    ONTO m1ypqynpsmlav2qlxd5rkutfirgskhuwjrvvuhedxu464j2hguk4qa
{
  ALTER TYPE default::FolderLike {
      CREATE MULTI LINK docs := (.<parent[IS default::Doc]);
      CREATE MULTI LINK files := (.<parent[IS default::File]);
      CREATE MULTI LINK folders := (.<parent[IS default::Folder]);
  };
};
