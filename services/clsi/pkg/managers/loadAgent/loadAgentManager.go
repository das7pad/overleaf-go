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

package loadAgent

import (
	"runtime"
	"sync"
	"syscall"
	"time"
)

type Manager interface {
	GetCapacity() (int64, error)
}

func New(refreshCapacityEvery time.Duration) Manager {
	nCores := float64(runtime.NumCPU())
	maxSystemLoad := nCores
	if nCores > 1 {
		maxSystemLoad -= 0.25
	}

	return &manager{
		maxSystemLoad:        maxSystemLoad,
		refreshCapacityEvery: refreshCapacityEvery,
		getCapacityExpiresAt: time.Now(),
	}
}

type manager struct {
	maxSystemLoad        float64
	refreshCapacityEvery time.Duration

	getCapacityMux       sync.Mutex
	getCapacityCapacity  int64
	getCapacityErr       error
	getCapacityExpiresAt time.Time
}

func (m *manager) GetCapacity() (int64, error) {
	m.getCapacityMux.Lock()
	defer m.getCapacityMux.Unlock()
	if m.getCapacityExpiresAt.After(time.Now()) {
		return m.getCapacityCapacity, m.getCapacityErr
	}
	capacity, err := m.refreshGetCapacity()
	m.getCapacityCapacity = capacity
	m.getCapacityErr = err
	m.getCapacityExpiresAt = time.Now().Add(m.refreshCapacityEvery)
	return capacity, err
}

const loadBase = 1 << 16

func (m *manager) refreshGetCapacity() (int64, error) {
	var info syscall.Sysinfo_t
	if err := syscall.Sysinfo(&info); err != nil {
		return 0, err
	}
	load1 := float64(info.Loads[0]) / loadBase
	capacity := int64(100 * (m.maxSystemLoad - load1) / m.maxSystemLoad)

	if capacity < 0 {
		capacity = 0
	}
	return capacity, nil
}
