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

CREATE MIGRATION m1b37ri3axxvkyvp5xarlx25a2vnwrxzczd4qufh2w634xl525rsza
    ONTO m1qhifrazxu6znemxemf3stleop6qkc7kebbwxqxyksc4s5vummbuq
{
  ALTER TYPE default::DocHistory {
      ALTER LINK doc {
          ON TARGET DELETE  DELETE SOURCE;
      };
  };
};
