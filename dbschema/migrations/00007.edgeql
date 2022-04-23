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

CREATE MIGRATION m1osu3te4fpztyvzqhocmycsc2udm27fl2aqkboo4xw4xs7kc2ja6q
    ONTO m1ypknroljbfg75ccyilrvcbrh3uemcuooyuyi2vnqsaznfpeygbpq
{
  ALTER TYPE default::Project {
      CREATE MULTI LINK all_files := (.<project[IS default::File]);
      ALTER LINK root_doc {
          RESET ON TARGET DELETE;
      };
  };
};
