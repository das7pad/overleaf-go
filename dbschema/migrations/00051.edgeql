CREATE MIGRATION m1etjt73mtr7bnxmokvhu6gskm65rdj5qopbud7raoitgea63fiuxa
    ONTO m1dg4t5qe4k2m47kmi5dnimr4vftmcrymd2sw4sw2bv67fdntmo6cq
{
  ALTER TYPE default::Project {
      CREATE LINK chat := (SELECT
          .<project[IS default::ChatRoom] 
      LIMIT
          1
      );
  };
};
