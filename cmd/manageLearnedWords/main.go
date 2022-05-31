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
	"flag"
	"log"
	"os"
	"time"

	"github.com/das7pad/overleaf-go/cmd/internal/utils"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

func main() {
	timeout := flag.Duration("timout", 10*time.Second, "timeout for operation")
	rawUserId := flag.String("user-id", "", "user id")
	deleteN := flag.Int("delete-n", 0, "enter number to delete")
	flag.Parse()

	userId, err := sharedTypes.ParseUUID(*rawUserId)
	if err != nil {
		flag.PrintDefaults()
		panic(errors.Tag(err, "cannot parse user id"))
	}

	db := utils.MustConnectPostgres(*timeout)
	um := user.New(db)
	ctx, done := context.WithTimeout(context.Background(), *timeout)
	defer done()

	log.Println("Getting users learned words.")
	u := user.LearnedWordsField{}
	if err = um.GetUser(ctx, userId, &u); err != nil {
		panic(errors.Tag(err, "cannot list users learned words"))
	}
	if len(u.LearnedWords) == 0 {
		log.Println("The user has no learned words.")
		return
	}

	switch flag.Arg(0) {
	case "", "list":
		log.Printf("The user has %d learned words.", len(u.LearnedWords))
		for i, word := range u.LearnedWords {
			log.Printf("%d: %q", i+1, word)
		}
	case "delete", "unlearn":
		word := flag.Arg(1)
		if word == "" && *deleteN == 0 {
			flag.PrintDefaults()
			panic(errors.New("specify word or number of words to delete"))
		}
		if word != "" {
			found := false
			for _, learnedWord := range u.LearnedWords {
				if learnedWord == word {
					found = true
					break
				}
			}
			if !found {
				log.Println("The word does not exist (anymore).")
				return
			}
			log.Println("Unlearning word.")
			if err = um.UnlearnWord(ctx, userId, word); err != nil {
				panic(errors.Tag(err, "cannot unlearn word"))
			}
		} else if *deleteN == len(u.LearnedWords) {
			log.Printf("Deleting all %d words.", len(u.LearnedWords))
			if err = um.DeleteDictionary(ctx, userId); err != nil {
				panic(errors.Tag(err, "cannot unlearn word"))
			}
		} else {
			log.Printf("Consider listing words first: %s list", os.Args[0])
			panic(errors.New("number of words to delete does not match"))
		}
		log.Println("NOTE: The user needs to reload all editor pages to see the changes.")
	}
	log.Println("done.")
}
