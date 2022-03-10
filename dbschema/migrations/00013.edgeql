CREATE MIGRATION m1z737mgdfhfiw4jnhc7wetqplhfo72bhsbpkavqxymu5cmkb7fzya
    ONTO m1lmmdaibcinydlgroprpbrq2mu52vp664ptx2fjhdrcpg5cuqkpmq
{
  ALTER TYPE default::OneTimeToken {
      CREATE PROPERTY used_at -> std::datetime;
  };
};
