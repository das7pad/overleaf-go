CREATE MIGRATION m1pqg2w4r7gbflgkbavor7fpg4dqhk6nyebjjrvlhhdcgm3rkzoefa
    ONTO m1qmhn5iyn2vud5ej5376smly65bd26nkvw4xigkzejosgs7lx4nqq
{
  ALTER TYPE default::Project {
      CREATE PROPERTY deleted_at -> std::datetime;
  };
};
