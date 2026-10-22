package common

// Placeholder inteface for now, might later move 'model' type to commons and have
// and add an execute(model) function to this
type ModelAction interface {
	String() string
}

type NoAction struct {
}

func (n NoAction) String() string {
	return "NoAction"
}

type ShellCommandAction struct {
	Command string
}

func (s ShellCommandAction) String() string {
	return "ShellCommandAction for command " + s.Command
}

// We could later move 'model' type to commons and have
// these actions implement an execute(model) interface
type SplitPanelAction struct{}

func (s SplitPanelAction) String() string {
	return "SplitPanelAction"
}

type CDCurrentPanelAction struct {
	Location string
}

func (c CDCurrentPanelAction) String() string {
	return "CDCurrentPanelAction to " + c.Location
}

type OpenPanelAction struct {
	Location string
}

func (o OpenPanelAction) String() string {
	return "OpenPanelAction at " + o.Location
}

type WorkspaceOperation string

const (
	WorkspaceSave   WorkspaceOperation = "save"
	WorkspaceLoad   WorkspaceOperation = "load"
	WorkspaceDelete WorkspaceOperation = "delete"
	WorkspaceList   WorkspaceOperation = "list"
)

// WorkspaceAction represents one explicit named-workspace operation. The
// prompt parses it, while the root model owns all persistence and UI updates.
type WorkspaceAction struct {
	Operation WorkspaceOperation
	Name      string
}

func (w WorkspaceAction) String() string {
	if w.Name == "" {
		return "WorkspaceAction " + string(w.Operation)
	}
	return "WorkspaceAction " + string(w.Operation) + " " + w.Name
}

type SearchOperation string

const (
	SearchSave   SearchOperation = "save"
	SearchRun    SearchOperation = "run"
	SearchDelete SearchOperation = "delete"
	SearchList   SearchOperation = "list"
)

type SearchAction struct {
	Operation SearchOperation
	Name      string
	Query     string
}

func (s SearchAction) String() string {
	return "SearchAction " + string(s.Operation) + " " + s.Name
}

type BatchRenameOperation string

const (
	BatchRenameReplace BatchRenameOperation = "replace"
	BatchRenamePrefix  BatchRenameOperation = "prefix"
	BatchRenameSuffix  BatchRenameOperation = "suffix"
	BatchRenameApply   BatchRenameOperation = "apply"
	BatchRenameCancel  BatchRenameOperation = "cancel"
)

type BatchRenameAction struct {
	Operation   BatchRenameOperation
	Find        string
	Replacement string
}

func (b BatchRenameAction) String() string {
	return "BatchRenameAction " + string(b.Operation)
}

type CatalogOperation string

const (
	TagAdd           CatalogOperation = "tag_add"
	TagRemove        CatalogOperation = "tag_remove"
	TagList          CatalogOperation = "tag_list"
	TagFind          CatalogOperation = "tag_find"
	CollectionSave   CatalogOperation = "collection_save"
	CollectionOpen   CatalogOperation = "collection_open"
	CollectionList   CatalogOperation = "collection_list"
	CollectionDelete CatalogOperation = "collection_delete"
)

type CatalogAction struct {
	Operation CatalogOperation
	Name      string
}

func (c CatalogAction) String() string {
	return "CatalogAction " + string(c.Operation) + " " + c.Name
}

type MetadataOperation string

const (
	MetadataModified    MetadataOperation = "modified"
	MetadataAccessed    MetadataOperation = "accessed"
	MetadataPermissions MetadataOperation = "permissions"
	MetadataApply       MetadataOperation = "apply"
	MetadataRestore     MetadataOperation = "restore"
)

type MetadataAction struct {
	Operation MetadataOperation
	Value     string
}

func (m MetadataAction) String() string {
	return "MetadataAction " + string(m.Operation)
}
