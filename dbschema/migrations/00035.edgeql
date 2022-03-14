CREATE MIGRATION m15xobmucnayblcxzpohfvz4arqbdjls5ml6iwggk4ii2jomnaku2q
    ONTO m1eogboap3gac3u36cmluqcaravfhzqkzbusurlfdhfww3jr2tfpla
{
  ALTER TYPE default::Project {
      CREATE MULTI LINK min_access_ro := (DISTINCT (((({.owner} UNION .access_token_ro) UNION .access_token_rw) UNION .members)));
      CREATE MULTI LINK min_access_rw := (DISTINCT ((({.owner} UNION .access_rw) UNION .access_token_rw)));
  };
};
