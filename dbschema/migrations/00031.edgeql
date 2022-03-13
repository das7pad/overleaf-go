CREATE MIGRATION m1fr3sfg3va72lnou73v5g5hnsvvupgy2gld6dq7q67lzzbjng2rea
    ONTO m16xo5whs4bry7crh6u32iitpjwmy2xbi2oy46yzmpqkmyo7wmrxoa
{
  ALTER TYPE default::Project {
      ALTER LINK token_access_ro {
          RENAME TO access_token_ro;
      };
  };
  ALTER TYPE default::Project {
      ALTER LINK token_access_rw {
          RENAME TO access_token_rw;
      };
  };
};
