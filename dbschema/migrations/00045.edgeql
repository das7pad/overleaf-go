CREATE MIGRATION m1bw7wumnh2hgil5db3pp5expqszjmm5e23ioku5y3h22e7knnzfrq
    ONTO m1iprcbylpobpa7eajs65ptmw6idyqsj3fjsccru5chdjnmzdoxmaa
{
  ALTER TYPE default::ProjectInvite {
      CREATE CONSTRAINT std::exclusive ON ((.project, .token));
  };
};
