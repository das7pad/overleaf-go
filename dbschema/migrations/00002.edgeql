CREATE MIGRATION m1pizgkosxeyp4amf7qlviihcgr7cilvmubd7p7tnfhnp3ijbvfnsq
    ONTO m1b7iy26ufukkc7xjgj4fmclvovog3caekvvegsdhx2nt4ggynl7ba
{
  ALTER TYPE default::Notification {
      CREATE CONSTRAINT std::exclusive ON ((.key, .user));
  };
};
