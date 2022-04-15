CREATE MIGRATION m1b37ri3axxvkyvp5xarlx25a2vnwrxzczd4qufh2w634xl525rsza
    ONTO m1qhifrazxu6znemxemf3stleop6qkc7kebbwxqxyksc4s5vummbuq
{
  ALTER TYPE default::DocHistory {
      ALTER LINK doc {
          ON TARGET DELETE  DELETE SOURCE;
      };
  };
};
