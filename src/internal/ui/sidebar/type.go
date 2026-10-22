package sidebar

import (
	"time"

	"charm.land/bubbles/v2/textinput"
)

type directory struct {
	Location string `json:"location"`
	Name     string `json:"name"`
	Section  string `json:"-"`
	Icon     string `json:"-"`
}

type Model struct {
	directories  []directory
	renderIndex  int
	renderOffset int // first visible row within renderIndex (section headings span rows)
	cursor       int
	rename       textinput.Model
	renaming     bool
	searchBar    textinput.Model
	pinnedMgr    *PinnedManager
	recentMgr    *RecentManager
	width        int
	height       int
	disabled     bool
	sections     []string
	lastRefresh  time.Time
	lastSearch   string
	refreshDirty bool
}
