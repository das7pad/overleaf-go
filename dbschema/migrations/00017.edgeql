CREATE MIGRATION m136fnnl3qohsejabjq7z75h7bmsn3a6wxlk4crcvtw5vgmjyhg4nq
    ONTO m15dakvei7jub4dzccm4qynjsixrdp2jrfnxawejzpp6mhqo5nlzaa
{
  ALTER TYPE default::Project {
      DROP PROPERTY track_changes_state;
  };
};
