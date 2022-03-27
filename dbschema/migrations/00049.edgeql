CREATE MIGRATION m1ojfc73ta5tzcqnwf2ird2vwiwcx457mlac3ndawdtnsrw5ndekpa
    ONTO m1sw5ssehspylr5z7cncgncjgbs4ukuqml4ttfn5qybdgyr25ef3ba
{
  ALTER TYPE default::FolderLike {
      CREATE REQUIRED PROPERTY path -> std::str {
          SET REQUIRED USING (SELECT
              ''
          );
      };
  };
};
