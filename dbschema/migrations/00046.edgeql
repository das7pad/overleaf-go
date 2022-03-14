CREATE MIGRATION m1qa5ydxltzla623hzcjp4wclkvm2xsidr3fbvqmptd32ab25zp2ya
    ONTO m1bw7wumnh2hgil5db3pp5expqszjmm5e23ioku5y3h22e7knnzfrq
{
  ALTER TYPE default::Project {
      ALTER LINK min_access_ro {
          USING (DISTINCT ((((({.owner} UNION .access_ro) UNION .access_rw) UNION .access_token_ro) UNION .access_token_rw)));
      };
  };
  ALTER TYPE default::Project {
      DROP LINK members;
  };
};
