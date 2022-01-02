// Golang port of Overleaf
// Copyright (C) 2022 Jakob Ackermann <das7pad@outlook.com>
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
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	_ "github.com/das7pad/overleaf-go/cmd/migrate/internal/migrations"
	"github.com/das7pad/overleaf-go/cmd/migrate/internal/register"
)

type Manager interface {
	List(ctx context.Context) bool
	Migrate(ctx context.Context) bool
}

func New(db *mongo.Database) Manager {
	return &manager{
		c:  db.Collection("migrations"),
		db: db,
	}
}

type manager struct {
	c  *mongo.Collection
	db *mongo.Database
}

type migration struct {
	Name       string    `bson:"name"`
	MigratedAt time.Time `bson:"migratedAt"`
}

func (m *manager) getMigrated(ctx context.Context) (map[string]time.Time, error) {
	r, err := m.c.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	var dst []migration
	if err = r.All(ctx, &dst); err != nil {
		return nil, err
	}
	out := make(map[string]time.Time, len(dst))
	for _, item := range dst {
		out[item.Name] = item.MigratedAt
	}
	return out, nil
}

func (m *manager) getOutstanding(ctx context.Context) ([]string, error) {
	migrated, err := m.getMigrated(ctx)
	if err != nil {
		return nil, err
	}
	all := register.List()
	todo := make([]string, 0, len(all))
	for _, name := range all {
		if _, exists := migrated[name]; exists {
			continue
		}
		todo = append(todo, name)
	}
	return todo, nil
}

func (m *manager) List(ctx context.Context) bool {
	log.Println("List: collecting outstanding migrations")
	todo, err := m.getOutstanding(ctx)
	if err != nil {
		log.Printf(
			"List: cannot list outstanding migrations: %s",
			err.Error(),
		)
		return false
	}
	n := len(todo)
	for i, name := range todo {
		i++
		log.Printf("List: %d/%d - %s", i, n, name)
	}
	return true
}

func (m *manager) Migrate(ctx context.Context) bool {
	log.Println("Migrate: collecting outstanding migrations")
	todo, err := m.getOutstanding(ctx)
	if err != nil {
		log.Printf(
			"Migrate: cannot list outstanding migrations: %s",
			err.Error(),
		)
		return false
	}
	n := len(todo)
	for i, name := range todo {
		i++
		log.Printf("Migrate: %d/%d - %s: started", i, n, name)
		if err = register.Run(ctx, name, m.db); err != nil {
			log.Printf(
				"Migrate: %d/%d, - %s: failed: %s",
				i, n, name, err.Error(),
			)
			return false
		}
		log.Printf("Migrate: %d/%d - %s: finished", i, n, name)
		_, err = m.c.InsertOne(
			ctx, migration{Name: name, MigratedAt: time.Now().UTC()},
		)
		if err != nil {
			log.Printf(
				"Migrate: cannot persist completion of %q: %s",
				name, err.Error(),
			)
			return false
		}
	}
	log.Printf("Migrate: all done.")
	return true
}
