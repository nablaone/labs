package main

import (
	"fmt"
	"leet1"
)

func main() {

	ll := leet1.N(22, nil, nil)
	lr := leet1.N(23, nil, nil)
	l := leet1.N(2, ll, lr)
	rl := leet1.N(32, nil, nil)
	r := leet1.N(3, rl, nil)
	root := leet1.N(1, l, r)

	root.Dump(0)

	serialized := root.Serialize()

	fmt.Println(serialized)

	newRoot := leet1.Deserialize(leet1.MakeByteStreamer(serialized))

	newRoot.Dump(0)

	fmt.Println(newRoot.Serialize())

	newRoot2 := leet1.Deserialize(leet1.MakeByteStreamer2(serialized))
	fmt.Println(newRoot2.Serialize())
}
