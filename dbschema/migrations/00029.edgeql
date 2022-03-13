CREATE MIGRATION m1mtwua7zrifejowydzjikjilnoyftu4d6434mxw25thexdpurghya
    ONTO m12nciqya3obgv37lyoq4m4tmb7dxrm4rj4lhkkru67hlo2cgxdzyq
{
  ALTER TYPE default::User {
      CREATE LINK projects := (((((.projects_owned UNION .projects_ro) UNION .projects_rw) UNION .projects_token_ro) UNION .projects_token_rw));
      CREATE LINK tags := (.<user[IS default::Tag]);
  };
};
