package main

import (
	"fmt"
	"sync"
)

func main() {
	var wg sync.WaitGroup
	buffer := make([]int, 10)
	for i := range buffer {
		wg.Add(1)
		go func(n int) {
			buffer[n%5] = n
			wg.Done()
		}(i)
	}

	wg.Wait()

	for i := range buffer {
		fmt.Println(i, buffer[i])
	}
}
