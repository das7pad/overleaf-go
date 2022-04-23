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

CREATE MIGRATION m15s5cjgoryneln4i26lqrtpoa4k6j26o7yagu64sotw5gzxckgv3a
    ONTO m1biug55fjsoaklcygrfkrzhbdlqbiwzkfla3zaqgze7fdwtdmdrma
{
  ALTER TYPE default::ProjectAuditLogEntry {
      CREATE REQUIRED LINK project -> default::Project {
          ON TARGET DELETE  DELETE SOURCE;
          SET REQUIRED USING (SELECT
              default::Project FILTER
                  (default::ProjectAuditLogEntry IN .audit_log)
          LIMIT
              1
          );
      };
  };
  ALTER TYPE default::UserAuditLogEntry {
      CREATE REQUIRED LINK user -> default::User {
          ON TARGET DELETE  DELETE SOURCE;
          SET REQUIRED USING (SELECT
              default::User FILTER
                  (default::UserAuditLogEntry IN .audit_log)
          LIMIT
              1
          );
      };
  };
};
