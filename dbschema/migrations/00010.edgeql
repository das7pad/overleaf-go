CREATE MIGRATION m1biug55fjsoaklcygrfkrzhbdlqbiwzkfla3zaqgze7fdwtdmdrma
    ONTO m1a52hgslfusbriuesm7z4tywewupzyij3cinkjhd6owe73aawofdq
{
  ALTER TYPE default::File {
      ALTER LINK linked_file_data {
          USING (SELECT
              .<file[IS default::LinkedFileData] 
          LIMIT
              1
          );
      };
  };
};
