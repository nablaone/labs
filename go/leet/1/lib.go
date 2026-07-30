package leet1

import (
	"fmt"
)

type Node struct {
	Value uint8
	Left  *Node
	Right *Node
}

func (n *Node) String() string {
	return fmt.Sprintf("(value: %d left: %v right: %v)", n.Value, n.Left, n.Right)
}

func (n *Node) Dump(indent int) {

	spaces := func(x int) {
		for i := 0; i < x; i = i + 1 {
			fmt.Print(" ")
		}
	}
	spaces(indent)
	fmt.Printf("{%d\n", n.Value)
	if n.Left == nil {
		spaces(indent + 1)
		fmt.Println("no left")
	} else {
		n.Left.Dump(indent + 1)
	}

	if n.Right == nil {
		spaces(indent + 1)
		fmt.Println("no right")
	} else {
		n.Right.Dump(indent + 1)
	}

	spaces(indent)
	fmt.Println("}")
}

func N(v uint8, l *Node, r *Node) *Node {
	return &Node{
		Value: v,
		Left:  l,
		Right: r,
	}
}

func (n *Node) Walk(fn func(*Node) bool) {

	if fn(n) {

		if n.Left != nil {
			n.Left.Walk(fn)
		} else {
			fn(nil)
		}

		if n.Right != nil {
			n.Right.Walk(fn)
		} else {
			fn(nil)
		}
	}
}

func (n *Node) Serialize() []uint8 {

	res := make([]uint8, 0)

	n.Walk(func(nn *Node) bool {

		if nn == nil {
			res = append(res, 0)
			return false
		}

		res = append(res, nn.Value)

		return true
	})

	return res

}

type ByteStreamer interface {
	next() uint8
}

type byteArrayByter struct {
	input []uint8
	i     int
}

func (b *byteArrayByter) next() uint8 {
	b.i = b.i + 1
	if b.i >= len(b.input) {
		return 0
	}
	return b.input[b.i]
}

func MakeByteStreamer(input []uint8) ByteStreamer {

	return &byteArrayByter{
		i:     -1,
		input: input,
	}
}

type byterFunc func() uint8

func (b byterFunc) next() uint8 {
	return b()
}

func MakeByteStreamer2(input []uint8) ByteStreamer {

	i := -1
	res := func() uint8 {
		i = i + 1
		if i >= len(input) {
			return 0
		}
		return input[i]
	}

	return byterFunc(res)
}

func Deserialize(input ByteStreamer) *Node {

	x := input.next()

	if x == 0 {
		return nil
	}

	res := N(x, nil, nil)

	res.Left = Deserialize(input)
	res.Right = Deserialize(input)

	return res
}
