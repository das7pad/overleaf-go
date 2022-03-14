CREATE MIGRATION m1iprcbylpobpa7eajs65ptmw6idyqsj3fjsccru5chdjnmzdoxmaa
    ONTO m1bnsddckkxb2l4hlop3rinqial3x2iqeljzecutfqfqw2lmil3dhq
{
  ALTER TYPE default::Project {
      CREATE MULTI LINK invites := (.<project[IS default::ProjectInvite]);
  };
};
