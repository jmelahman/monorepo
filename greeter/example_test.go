package greeter_test

import (
	"fmt"
	"log"

	"github.com/jmelahman/golang-template/greeter"
)

// Example functions compile as tests and render on pkg.go.dev, so they
// cannot rot. Name them Example, ExampleT, or ExampleT_method.
func ExampleGreeter_Greet() {
	msg, err := greeter.New(greeter.WithGreeting("Howdy")).Greet("world")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(msg)
	// Output: Howdy, world!
}
