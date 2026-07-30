package leet1_test

import (
	"leet1"
	"testing"
)

func TestSerializeEmpty(t *testing.T) {
	given := leet1.N(1, nil, nil)

	b := given.Serialize()

	if len(b) != 3 {
		t.Fatal("must be 3")
	}

	expected := []uint8{1, 0, 0}

	for i, v := range expected {
		if v != b[i] {
			t.Fatalf("index %d got %d expected %d", i, b[i], v)
		}
	}
}

func TestSerialize2(t *testing.T) {

	l := leet1.N(2, nil, nil)
	r := leet1.N(3, nil, nil)
	given := leet1.N(1, l, r)

	b := given.Serialize()

	if len(b) != 7 {
		t.Fatalf("must be 3 but got: %d", len(b))
	}

	expected := []uint8{1, 2, 0, 0, 3, 0, 0}

	for i, v := range expected {
		if v != b[i] {
			t.Fatalf("index %d got %d expected %d", i, b[i], v)
		}
	}
}

func TestDerializeEmpty(t *testing.T) {

	given := []uint8{0}

	node := leet1.Deserialize(leet1.MakeByteStreamer(given))

	if node != nil {
		t.Fatal("must be empty")
	}

}

func TestDerializeSingle(t *testing.T) {

	given := []uint8{1, 0, 0}

	node := leet1.Deserialize(leet1.MakeByteStreamer(given))

	if node == nil {
		t.Fatal("must not be empty")
	}

	if node.Value != 1 {
		t.Fatal("must be 1")
	}
}

func TestDerializeSingleByter2(t *testing.T) {

	given := []uint8{1, 0, 0}

	node := leet1.Deserialize(leet1.MakeByteStreamer2(given))

	if node == nil {
		t.Fatal("must not be empty")
	}

	if node.Value != 1 {
		t.Fatal("must be 1")
	}
}
