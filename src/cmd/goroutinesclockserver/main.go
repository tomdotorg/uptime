package main

import (
	"fmt"
	"time"
)

func main() {
	start := time.Now()
	go spinner(100 * time.Millisecond)
	const n = 45
	fibN := fib(n) // slow
	println("\rFibonacci(%d) = %d (%v)\n", n, fibN, time.Since(start))
}

func spinner(delay time.Duration) {
	for {
		for _, r := range `-\|/` {
			fmt.Printf("\r%c", r)
			time.Sleep(delay)
		}
	}
}

func fib(x int) int {
	if x < 2 {
		return x
	}
	return fib(x-1) + fib(x-2)
}
