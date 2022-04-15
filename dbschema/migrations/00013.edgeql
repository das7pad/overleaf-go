CREATE MIGRATION m1csxr3tw62zgni7fokf3223w4jw2rbvxfqbfgvpcq46itddbmlvcq
    ONTO m1qnddlsbajpnboiy7pzk5tzxg4mry4iiledqhzwxjgsh5q6b56mwa
{
  ALTER TYPE default::Message {
      ALTER LINK user {
          ON TARGET DELETE  ALLOW;
          RESET OPTIONALITY;
      };
  };
};
