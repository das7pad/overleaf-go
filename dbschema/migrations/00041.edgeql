CREATE MIGRATION m136ut7idfaboyjedrpn7nlwvsponwq3nc7oubog37qasazckpfowa
    ONTO m1za5tqcioqf72jvyod3bl4w3hrfcok23ktfdea6q4pckgfbfqoykq
{
  CREATE TYPE default::EditorConfig {
      CREATE REQUIRED PROPERTY auto_complete -> std::bool;
      CREATE REQUIRED PROPERTY auto_pair_delimiters -> std::bool;
      CREATE REQUIRED PROPERTY font_family -> std::str;
      CREATE REQUIRED PROPERTY font_size -> std::int64;
      CREATE REQUIRED PROPERTY line_height -> std::str;
      CREATE REQUIRED PROPERTY mode -> std::str;
      CREATE REQUIRED PROPERTY overall_theme -> std::str;
      CREATE REQUIRED PROPERTY pdf_viewer -> std::str;
      CREATE REQUIRED PROPERTY spell_check_language -> std::str;
      CREATE REQUIRED PROPERTY syntax_validation -> std::bool;
      CREATE REQUIRED PROPERTY theme -> std::str;
  };
  ALTER TYPE default::User {
      CREATE LINK editor_config -> default::EditorConfig;
  };
};
