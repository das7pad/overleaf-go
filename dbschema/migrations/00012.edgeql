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

CREATE MIGRATION m1qnddlsbajpnboiy7pzk5tzxg4mry4iiledqhzwxjgsh5q6b56mwa
    ONTO m15s5cjgoryneln4i26lqrtpoa4k6j26o7yagu64sotw5gzxckgv3a
{
  ALTER TYPE default::Project {
      ALTER LINK audit_log {
          USING (.<project[IS default::ProjectAuditLogEntry]);
          RESET ON TARGET DELETE;
      };
  };
  ALTER TYPE default::User {
      ALTER LINK audit_log {
          USING (.<user[IS default::UserAuditLogEntry]);
          RESET ON TARGET DELETE;
      };
  };
};
