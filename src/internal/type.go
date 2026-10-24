package internal

import (
	"context"
	"sync"
	"time"

	zoxidelib "github.com/lazysegtree/go-zoxide"

	"github.com/yorukot/superfile/src/internal/batchrename"
	"github.com/yorukot/superfile/src/internal/catalog"
	"github.com/yorukot/superfile/src/internal/folderwatch"
	"github.com/yorukot/superfile/src/internal/gitstatus"
	"github.com/yorukot/superfile/src/internal/metadataedit"
	"github.com/yorukot/superfile/src/internal/plugincommands"
	"github.com/yorukot/superfile/src/internal/savedsearch"
	"github.com/yorukot/superfile/src/internal/session"
	"github.com/yorukot/superfile/src/internal/ui/helpmenu"
	"github.com/yorukot/superfile/src/internal/ui/spferror"

	"github.com/yorukot/superfile/src/internal/ui/archiveviewer"
	"github.com/yorukot/superfile/src/internal/ui/clipboard"
	"github.com/yorukot/superfile/src/internal/ui/commandpalette"
	"github.com/yorukot/superfile/src/internal/ui/remoteviewer"
	"github.com/yorukot/superfile/src/internal/ui/sortmodel"
	"github.com/yorukot/superfile/src/internal/ui/textviewer"

	"github.com/yorukot/superfile/src/internal/ui/filemodel"
	"github.com/yorukot/superfile/src/internal/ui/globalsearch"

	"github.com/yorukot/superfile/src/internal/ui/metadata"
	"github.com/yorukot/superfile/src/internal/ui/notify"
	"github.com/yorukot/superfile/src/internal/ui/processbar"
	"github.com/yorukot/superfile/src/internal/ui/sidebar"

	"charm.land/bubbles/v2/textinput"

	"github.com/yorukot/superfile/src/internal/ui/prompt"
	zoxideui "github.com/yorukot/superfile/src/internal/ui/zoxide"
)

// Type representing the type of focused panel
type focusPanelType int

type modelQuitStateType int

// Constants for panel with no focus
const (
	nonePanelFocus focusPanelType = iota
	processBarFocus
	sidebarFocus
	metadataFocus
)

const (
	notQuitting modelQuitStateType = iota
	quitInitiated
	quitConfirmationInitiated
	quitConfirmationReceived
	quitDone
)

// Main model
// TODO : We could consider using *model as tea.Model, instead of model.
// for reducing re-allocations. The struct is 20K bytes. But this could lead to
// issues like race conditions and whatnot, which are hidden since we are creating
// new model in each tea update.
type model struct {
	// Main Panels
	fileModel           filemodel.Model
	sidebarModel        sidebar.Model
	processBarModel     processbar.Model
	operations          operationCenter
	operationsStatePath string
	clipboard           clipboard.Model
	clipboardWriter     func(string) error
	focusPanel          focusPanelType

	// Modals
	notifyModel          notify.Model
	typingModal          typingModal
	helpMenu             helpmenu.Model
	promptModal          prompt.Model
	zoxideModal          zoxideui.Model
	sortModal            sortmodel.Model
	spfError             spferror.Model
	mutexErrorModal      sync.Mutex
	lastUndo             *undoAction
	mouse                mouseState
	contextMenu          contextMenuState
	globalSearchModal    globalsearch.Model
	commandPalette       commandpalette.Model
	textViewer           textviewer.Model
	archiveViewer        archiveviewer.Model
	remoteViewer         remoteviewer.Model
	sessionStore         *session.Store
	savedSearchStore     *savedsearch.Store
	catalogModel         *catalog.Catalog
	folderWatcher        folderwatch.Service
	folderWatchPaths     []string
	gitClient            *gitstatus.Client
	gitStatusPaths       []string
	gitStatusReqID       int
	gitStatusLast        time.Time
	pendingGitDiscard    []string
	pendingBatchRename   *batchrename.Plan
	duplicateScanCancel  context.CancelFunc
	duplicateScanReqID   int
	pendingDroppedPaths  []string
	dirCompareCancel     context.CancelFunc
	dirCompareReqID      int
	pendingSync          *pendingDirectorySync
	pendingMetadataPatch *metadataedit.Patch
	lastMetadataJournal  string
	remoteClient         remoteService
	remoteCancel         context.CancelFunc
	remoteReqID          int
	pluginCommands       *plugincommands.Registry
	pluginCommandDir     string
	pluginCommandCancel  context.CancelFunc
	pluginCommandReqID   int

	// Zoxide client for directory tracking
	zClient *zoxidelib.Client

	fileMetaData metadata.Model

	// no use directly for increment, use nextIoReqCnt
	ioReqCnt int32

	modelQuitState       modelQuitStateType
	firstTextInput       bool
	toggleFooter         bool
	firstLoadingComplete bool
	firstUse             bool

	// This entirely disables metadata fetching. Used in test model
	disableMetadata bool
	metadataCancel  context.CancelFunc
	metadataReqID   int

	// Height in number of lines of actual viewport of
	// main panel and sidebar excluding border
	mainPanelHeight int

	// Height in number of lines of actual viewport of
	// footer panels - process/metadata/clipboard - excluding border
	footerHeight int
	fullWidth    int
	fullHeight   int

	// whether usable trash directory exists or not
	hasTrash bool
}

type typingModal struct {
	location  string
	open      bool
	textInput textinput.Model
}

type editorFinishedMsg struct{ err error }

type mouseState struct {
	lastPanel int
	lastPath  string
	lastClick time.Time
}
