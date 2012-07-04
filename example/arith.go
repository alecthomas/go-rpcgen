package arith

type Arith interface {
	Add(a, b int) (result int, err error)
}
