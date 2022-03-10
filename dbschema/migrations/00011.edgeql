CREATE MIGRATION m1y777npfc5s4ym3zia45uofh7puq2hsp2h2egeautncfkaetu3f2a
    ONTO m1p3hfsveourfscs4xi7axl2jxwxsl5sklbfqv2tdnggjqwdnbod5q
{
  ALTER TYPE default::OneTimeToken {
      ALTER PROPERTY created_at {
          SET default := (std::datetime_of_transaction());
      };
  };
};
