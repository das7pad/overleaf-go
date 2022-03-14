CREATE MIGRATION m1mdnn4oogexeapl66lf6fqowqnij56tkibsjuxe7qdienqkfcwk4q
    ONTO m15jlupdvdls5cz3fl6ermdhayib64jqeq7uxo222l7ume4ie5udpq
{
  ALTER TYPE default::Project {
      ALTER PROPERTY track_changes {
          RENAME TO track_changes_state;
      };
  };
};
