CREATE MIGRATION m1lasydn7f7dddkjxdgeh7mslyz3rnkvysj7npw7qcbjmwru6ie5hq
    ONTO m1few24crlv7sd6guattj7gq7d5wna7w6kogscxmalp4a5vnvewv5q
{
  ALTER TYPE default::Project {
      ALTER LINK rootDoc {
          RENAME TO root_doc;
      };
  };
};
