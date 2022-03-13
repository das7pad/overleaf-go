CREATE MIGRATION m16xo5whs4bry7crh6u32iitpjwmy2xbi2oy46yzmpqkmyo7wmrxoa
    ONTO m1mtwua7zrifejowydzjikjilnoyftu4d6434mxw25thexdpurghya
{
  ALTER TYPE default::User {
      ALTER LINK projects {
          USING (DISTINCT (((((.projects_owned UNION .projects_ro) UNION .projects_rw) UNION .projects_token_ro) UNION .projects_token_rw)));
      };
  };
};
