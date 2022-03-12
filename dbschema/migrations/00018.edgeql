CREATE MIGRATION m1mwgq4x4wfbb35ar2kssxecfuuz6c4eaidoqlx7r22h5lfqxemxiq
    ONTO m17rxftmdko77h7fatzoutxuoqjy6veckg6cc6xjin2cgkxlx6eqfa
{
  ALTER TYPE default::Features {
      DROP PROPERTY name;
      DROP PROPERTY versioning;
  };
};
