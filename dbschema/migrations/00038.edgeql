CREATE MIGRATION m15jlupdvdls5cz3fl6ermdhayib64jqeq7uxo222l7ume4ie5udpq
    ONTO m14mlth7nlf7wmxgtv4ijmjtoztrdid666ulvz4kr7kr73dj6wruxq
{
  ALTER TYPE default::Project {
      CREATE REQUIRED PROPERTY track_changes -> std::json {
          SET default := (std::to_json('{}'));
      };
  };
};
