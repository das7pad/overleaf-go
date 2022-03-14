CREATE MIGRATION m14mlth7nlf7wmxgtv4ijmjtoztrdid666ulvz4kr7kr73dj6wruxq
    ONTO m1im67b3dwdjih4pbwslvayyl25sdsczc56trzocmmzj42zeqcrf7a
{
  ALTER TYPE default::Project {
      DROP PROPERTY track_changes;
  };
};
