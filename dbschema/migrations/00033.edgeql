CREATE MIGRATION m1v5yjtc4sxq4rohh53u5ks2m65mm5tazbacjuidmsmk2llfkzuhzq
    ONTO m1iep3y5q5q3uxbo76fqqsqojs6hrdmaooc6louwgqhq43ayfj4xjq
{
  ALTER TYPE default::Project {
      ALTER LINK trashed {
          RENAME TO trashed_by;
      };
  };
};
