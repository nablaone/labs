package main

//go:generate go-papagen intmin/min.go uint8min T=uint8
//go:generate go-papagen intmin/min.go floatmin T=float32

import (
	"fmt"

	"github.com/nablaone/go-papagen/example/floatmin"
	"github.com/nablaone/go-papagen/example/intmin"
	"github.com/nablaone/go-papagen/example/uint8min"
)

func main() {

	fmt.Println(intmin.Min(1, 2))
	fmt.Println(floatmin.Min(1.0, 2.0))
	fmt.Println(uint8min.Min(1, 2))

}
