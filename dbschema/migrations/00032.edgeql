CREATE MIGRATION m1iep3y5q5q3uxbo76fqqsqojs6hrdmaooc6louwgqhq43ayfj4xjq
    ONTO m1fr3sfg3va72lnou73v5g5hnsvvupgy2gld6dq7q67lzzbjng2rea
{
  ALTER TYPE default::Project {
      CREATE LINK members := (DISTINCT ((.access_ro UNION .access_rw)));
  };
};
