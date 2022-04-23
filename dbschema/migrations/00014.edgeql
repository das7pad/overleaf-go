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

CREATE MIGRATION m1qhifrazxu6znemxemf3stleop6qkc7kebbwxqxyksc4s5vummbuq
    ONTO m1csxr3tw62zgni7fokf3223w4jw2rbvxfqbfgvpcq46itddbmlvcq
{
  CREATE TYPE default::DocHistory {
      CREATE REQUIRED LINK doc -> default::Doc;
      CREATE LINK user -> default::User {
          ON TARGET DELETE  ALLOW;
      };
      CREATE REQUIRED PROPERTY end_at -> std::datetime;
      CREATE REQUIRED PROPERTY op -> std::json;
      CREATE REQUIRED PROPERTY start_at -> std::datetime;
      CREATE REQUIRED PROPERTY version -> std::int64;
  };
};
