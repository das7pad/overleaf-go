// Golang port of Overleaf
// Copyright (C) 2021-2022 Jakob Ackermann <das7pad@outlook.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"github.com/das7pad/overleaf-go/pkg/options/listenAddress"
	"github.com/das7pad/overleaf-go/pkg/options/utils"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

type clsiOptions struct {
	address string
	options types.Options

	loadAddress  string
	loadShedding bool

	copyExecAgent    bool
	copyExecAgentSrc string
	copyExecAgentDst string
}

func getOptions() *clsiOptions {
	o := &clsiOptions{}

	o.options.FillFromEnv("OPTIONS")
	o.address = listenAddress.Parse(3013)

	loadPort := utils.GetIntFromEnv("LOAD_PORT", 3048)
	o.loadAddress = listenAddress.Parse(loadPort)
	o.loadShedding = utils.GetStringFromEnv("LOAD_SHEDDING", "false") == "true"

	o.copyExecAgentSrc = utils.GetStringFromEnv("COPY_EXEC_AGENT_SRC", "")
	o.copyExecAgentDst = utils.GetStringFromEnv("COPY_EXEC_AGENT_DST", "")
	o.copyExecAgent = utils.GetStringFromEnv("COPY_EXEC_AGENT", "false") == "true"

	return o
}
