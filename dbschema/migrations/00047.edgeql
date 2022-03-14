CREATE MIGRATION m1o77tbdl7tfcdnvuudwq4jqjfn2c7tgqu34zytnbkgykpztzq3oya
    ONTO m1qa5ydxltzla623hzcjp4wclkvm2xsidr3fbvqmptd32ab25zp2ya
{
  ALTER TYPE default::FolderLike {
      ALTER LINK docs {
          USING (SELECT
              .<parent[IS default::Doc]
          FILTER
              NOT (EXISTS (.deleted_at))
          );
      };
  };
  ALTER TYPE default::FolderLike {
      ALTER LINK files {
          USING (SELECT
              .<parent[IS default::File]
          FILTER
              NOT (EXISTS (.deleted_at))
          );
      };
  };
  ALTER TYPE default::FolderLike {
      ALTER LINK folders {
          USING (SELECT
              .<parent[IS default::Folder]
          FILTER
              NOT (EXISTS (.deleted_at))
          );
      };
  };
  ALTER TYPE default::Project {
      DROP LINK any_folders;
  };
  ALTER TYPE default::Project {
      CREATE MULTI LINK deleted_docs := (SELECT
          .<project[IS default::Doc]
      FILTER
          EXISTS (.deleted_at)
      );
  };
  ALTER TYPE default::Project {
      ALTER LINK docs {
          USING (SELECT
              .<project[IS default::Doc]
          FILTER
              NOT (EXISTS (.deleted_at))
          );
      };
  };
  ALTER TYPE default::Project {
      ALTER LINK files {
          USING (SELECT
              .<project[IS default::File]
          FILTER
              NOT (EXISTS (.deleted_at))
          );
      };
  };
  ALTER TYPE default::Project {
      ALTER LINK folders {
          USING (SELECT
              .<project[IS default::Folder]
          FILTER
              NOT (EXISTS (.deleted_at))
          );
      };
  };
};
