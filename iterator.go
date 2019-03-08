package urkel

// Far from finished...
type iteratorState struct {
	Item  node
	Depth uint
	Child int
}

/*type Iterator struct {
	tree  *Tree
	root  node
	stack []iteratorState
	stop  bool
	item  node
}

func NewIterator(t *Tree) *Iterator {
	return &Iterator{
		tree:  t,
		root:  t.Root,
		stack: make([]iteratorState, 0),
		stop:  false,
		item:  &nullNode{},
	}
}

func (it *Iterator) push(n node, depth uint) {
	is := iteratorState{
		Item:  n,
		Depth: depth,
		Child: -1,
	}
	it.stack = append(it.stack, is)
}

// Remove the latest from the stack
func (it *Iterator) pop() {
	it.stack = it.stack[:len(it.stack)-1]
}

// Returns the latest w/o removing it
func (it *Iterator) latest() iteratorState {
	return it.stack[len(it.stack)-1]
}

func (it *Iterator) traverse() bool {
	return true
}

func (it *Iterator) Next() {}*/
