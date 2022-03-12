CREATE MIGRATION m1qyii5b6bkcufatu7zns7i3zk6bty4z4dfpys5e73n3ymqy565qta
    ONTO m1ozrfhyjr75najc7zhp7wfqk54wzpyniejc4shj4lxhgose3rs2hq
{
  ALTER TYPE default::Email {
      CREATE LINK user := (.<emails[IS default::User]);
  };
};
