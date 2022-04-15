CREATE MIGRATION m1qhifrazxu6znemxemf3stleop6qkc7kebbwxqxyksc4s5vummbuq
    ONTO m1csxr3tw62zgni7fokf3223w4jw2rbvxfqbfgvpcq46itddbmlvcq
{
  CREATE TYPE default::DocHistory {
      CREATE REQUIRED LINK doc -> default::Doc;
      CREATE LINK user -> default::User {
          ON TARGET DELETE  ALLOW;
      };
      CREATE REQUIRED PROPERTY end_at -> std::datetime;
      CREATE REQUIRED PROPERTY op -> std::json;
      CREATE REQUIRED PROPERTY start_at -> std::datetime;
      CREATE REQUIRED PROPERTY version -> std::int64;
  };
};
