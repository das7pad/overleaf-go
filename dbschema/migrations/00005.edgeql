CREATE MIGRATION m1m2igb575cksepg3myqlf2ybwnvtwwzaqipwygdq6x2gl247hekaq
    ONTO m1pqg2w4r7gbflgkbavor7fpg4dqhk6nyebjjrvlhhdcgm3rkzoefa
{
  ALTER TYPE default::Doc {
      DROP PROPERTY in_s3;
  };
  ALTER TYPE default::Project {
      DROP PROPERTY active;
  };
  ALTER TYPE default::User {
      CREATE PROPERTY deleted_at -> std::datetime;
  };
};
