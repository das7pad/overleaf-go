# Golang port of Overleaf
# Copyright (C) 2022 Jakob Ackermann <das7pad@outlook.com>
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU Affero General Public License as published
# by the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU Affero General Public License for more details.
#
# You should have received a copy of the GNU Affero General Public License
# along with this program.  If not, see <https://www.gnu.org/licenses/>.

CREATE MIGRATION m1elontv6ocznarlr5h26p7ti2pt37om473gdkmb3i73en6aktdl3a
    ONTO m1bv5sb33s2wzs233ixyniobpzuda2oc5gn3s6prg2kalw5ry7pj5a
{
  CREATE TYPE default::LinkedFileData {
      CREATE REQUIRED PROPERTY provider -> std::str;
      CREATE PROPERTY source_entity_path -> std::str;
      CREATE PROPERTY source_output_file_path -> std::str;
      CREATE PROPERTY source_project_id -> std::str;
      CREATE PROPERTY url -> std::str;
  };
  ALTER TYPE default::File {
      CREATE LINK linked_file_data -> default::LinkedFileData;
  };
  DROP TYPE default::LinkedFileProjectFile;
  DROP TYPE default::LinkedFileProjectOutputFile;
  DROP TYPE default::LinkedFileURL;
};
