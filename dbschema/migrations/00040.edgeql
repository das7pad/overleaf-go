CREATE MIGRATION m1za5tqcioqf72jvyod3bl4w3hrfcok23ktfdea6q4pckgfbfqoykq
    ONTO m1mdnn4oogexeapl66lf6fqowqnij56tkibsjuxe7qdienqkfcwk4q
{
  ALTER TYPE default::User {
      DROP PROPERTY editor_config;
  };
};
