CREATE MIGRATION m1dg4t5qe4k2m47kmi5dnimr4vftmcrymd2sw4sw2bv67fdntmo6cq
    ONTO m1ojfc73ta5tzcqnwf2ird2vwiwcx457mlac3ndawdtnsrw5ndekpa
{
  ALTER TYPE default::FolderLike {
      CREATE PROPERTY path_for_join := (('' IF (.path = '') ELSE (.path ++ '/')));
  };
};
