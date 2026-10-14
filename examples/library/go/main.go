// Use cuppa as a Go library. Run from the repo root:
//
//	go run ./examples/library/go
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/kashifkhan0771/cuppa/pkg/cuppa"
)

func main() {
	// the questions built into the library, narrowed to two categories
	questions := cuppa.Embedded().Filter(cuppa.Filter{
		Categories: []string{"icebreakers", "this-or-that"},
		SFW:        "casual", // safe plus casual
	})

	// any question
	random, err := cuppa.Random(questions, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("random:", random.Text)

	// the question of the day: no saved state, same answer on every machine
	today, err := cuppa.Daily(questions, "my-team", time.Now())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("today: ", today.Text)

	// no repeats until every question was used. MemoryHistory forgets when
	// the program stops; implement cuppa.History to keep it on disk.
	history := &cuppa.MemoryHistory{}
	for range 3 {
		next, err := cuppa.Next(questions, history, nil)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("next:  ", next.Text)
	}
}
