package main

import "fmt"

type A1 struct {
	A string
	X *X1
}

type A2 struct {
	A string
	X *X2
}

type X1 struct {
	X string
}

type X2 struct {
	X string
}

func main() {
	a1 := &A1{"t", nil}

	a1x := a1.X
	a1str := a1.A

	a2 := (*A2)(&A2{a1x, a1str})

	fmt.Println(a2)
}
