package main

import (
	"testing"
)

func BenchmarkActor(b *testing.B) {
	log.Debugf("About to run main() %d times", b.N)
	for i := 0; i < b.N; i++ {
		main()
	}
}
