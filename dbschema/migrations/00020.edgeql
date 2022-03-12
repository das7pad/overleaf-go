CREATE MIGRATION m1setqfjw335jin2zezhilsa5e3xdvr5mi5cyhwdzpofetikg74ema
    ONTO m1w5e2ty6e7hpvgayfks77eyrwb7m6gpkzykq2rhw4l7dno5sn477q
{
  ALTER TYPE default::UserAuditLogEntry {
      ALTER PROPERTY timestamp {
          SET default := (std::datetime_of_transaction());
      };
  };
};
