package intmin


type T = int


func Min(a, b T) T {
	if a < b {
		return a
	}
	return b
}


