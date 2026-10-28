package sortmodel

type SortKind int

// NOTE: Update the validation of DefaultSortType config if you make changes here
const (
	SortByName SortKind = iota
	SortBySize
	SortByDate
	SortByType
	SortByNatural
)

var SortOptionsStr = []string{ //nolint: gochecknoglobals // Effectively const
	"Name", "Date modified", "Type", "Size", "Natural", "Ascending", "Descending",
}

var SortOptionsShortStr = []string{ //nolint: gochecknoglobals // Effectively const
	"Name", "Size", "Date", "Type", "Natural",
}

// Sort options
type Model struct {
	Width  int
	Height int
	open   bool

	// Cursor has meaning only during open state, its lost on close
	Cursor       int
	SelectedKind SortKind
	Reversed     bool
}

//nolint:gochecknoglobals // Static display order; persisted enums stay unchanged.
var menuKinds = [...]SortKind{
	SortByName,
	SortByDate,
	SortByType,
	SortBySize,
	SortByNatural,
}
