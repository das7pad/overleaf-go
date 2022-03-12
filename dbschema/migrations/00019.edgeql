CREATE MIGRATION m1w5e2ty6e7hpvgayfks77eyrwb7m6gpkzykq2rhw4l7dno5sn477q
    ONTO m1mwgq4x4wfbb35ar2kssxecfuuz6c4eaidoqlx7r22h5lfqxemxiq
{
  ALTER TYPE default::Email {
      ALTER PROPERTY preference {
          SET default := 0;
      };
  };
};
