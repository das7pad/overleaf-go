CREATE MIGRATION m1yykhqk45u42jkagjyrofuj2anbsvyhevpcajbfzqx2ztaedvwrqa
    ONTO m1etjt73mtr7bnxmokvhu6gskm65rdj5qopbud7raoitgea63fiuxa
{
  ALTER TYPE default::ContentElement {
      CREATE PROPERTY resolved_path := ((.parent.path_for_join ++ .name));
  };
};
