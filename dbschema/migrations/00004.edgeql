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

CREATE MIGRATION m1pqg2w4r7gbflgkbavor7fpg4dqhk6nyebjjrvlhhdcgm3rkzoefa
    ONTO m1qmhn5iyn2vud5ej5376smly65bd26nkvw4xigkzejosgs7lx4nqq
{
  ALTER TYPE default::Project {
      CREATE PROPERTY deleted_at -> std::datetime;
  };
};
