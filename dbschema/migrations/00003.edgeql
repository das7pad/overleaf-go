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

CREATE MIGRATION m1qmhn5iyn2vud5ej5376smly65bd26nkvw4xigkzejosgs7lx4nqq
    ONTO m1elontv6ocznarlr5h26p7ti2pt37om473gdkmb3i73en6aktdl3a
{
  ALTER TYPE default::LinkedFileData {
      ALTER PROPERTY source_entity_path {
          SET REQUIRED USING (SELECT
              ''
          );
      };
  };
  ALTER TYPE default::LinkedFileData {
      ALTER PROPERTY source_output_file_path {
          SET REQUIRED USING (SELECT
              ''
          );
      };
  };
  ALTER TYPE default::LinkedFileData {
      ALTER PROPERTY source_project_id {
          SET REQUIRED USING (SELECT
              ''
          );
      };
  };
  ALTER TYPE default::LinkedFileData {
      ALTER PROPERTY url {
          SET REQUIRED USING (SELECT
              ''
          );
      };
  };
};
