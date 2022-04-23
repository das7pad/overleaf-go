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

CREATE MIGRATION m1ypknroljbfg75ccyilrvcbrh3uemcuooyuyi2vnqsaznfpeygbpq
    ONTO m1m2igb575cksepg3myqlf2ybwnvtwwzaqipwygdq6x2gl247hekaq
{
  ALTER TYPE default::Project {
      ALTER LINK owner {
          RESET ON TARGET DELETE;
      };
  };
  ALTER TYPE default::ProjectAuditLogEntry {
      ALTER LINK initiator {
          ON TARGET DELETE  ALLOW;
          RESET OPTIONALITY;
      };
  };
  CREATE TYPE default::ProjectInviteNotification EXTENDING default::Notification {
      CREATE REQUIRED LINK project_invite -> default::ProjectInvite {
          ON TARGET DELETE  DELETE SOURCE;
      };
  };
  DROP TYPE default::ReviewThread;
  ALTER TYPE default::User {
      ALTER LINK contacts {
          ON TARGET DELETE  ALLOW;
      };
  };
  ALTER TYPE default::UserAuditLogEntry {
      ALTER LINK initiator {
          ON TARGET DELETE  ALLOW;
          RESET OPTIONALITY;
      };
  };
};
