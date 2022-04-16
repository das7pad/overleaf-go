CREATE MIGRATION m15dakvei7jub4dzccm4qynjsixrdp2jrfnxawejzpp6mhqo5nlzaa
    ONTO m1b37ri3axxvkyvp5xarlx25a2vnwrxzczd4qufh2w634xl525rsza
{
  ALTER TYPE default::DocHistory {
      CREATE REQUIRED PROPERTY has_big_delete -> std::bool {
          SET REQUIRED USING (SELECT
              false
          );
      };
  };
};
