CREATE MIGRATION m1lmmdaibcinydlgroprpbrq2mu52vp664ptx2fjhdrcpg5cuqkpmq
    ONTO m1y777npfc5s4ym3zia45uofh7puq2hsp2h2egeautncfkaetu3f2a
{
  ALTER TYPE default::OneTimeToken {
      DROP PROPERTY used_at;
  };
};
