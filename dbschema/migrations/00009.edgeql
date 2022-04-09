CREATE MIGRATION m1a52hgslfusbriuesm7z4tywewupzyij3cinkjhd6owe73aawofdq
    ONTO m1lzo6rdilxhtitxxrx5nvezvu2torphkg7kpsmdeqkgatq4exzgoa
{
  ALTER TYPE default::File {
      ALTER LINK linked_file_data {
          USING (.<file[IS default::LinkedFileData]);
      };
  };
};
