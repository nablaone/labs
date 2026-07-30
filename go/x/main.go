package main

import "fmt"

type Interface interface {
	Foo()
	Bar()
}

type Base struct {
}

func (b Base) Foo() {
	fmt.Println("Base.Foo")
	b.Bar()
}

func (b Base) Bar() {
	fmt.Println("Base.Bar")
}

type Extended struct {
	Base
}

func (b Extended) Bar() {
	fmt.Println("Extended.Bar")
}

func main() {

	b := Base{}
	b.Foo()

	e := Extended{}
	e.Foo()
}
