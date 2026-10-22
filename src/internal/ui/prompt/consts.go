package prompt

import "time"

// These could as well be property of prompt Model vs being global consts
// But its fine
const (
	promptHeadlineText = "Xelif Prompt"

	OpenCommand        = "open"
	SplitCommand       = "split"
	CdCommand          = "cd"
	WorkspaceCommand   = "workspace"
	SearchCommand      = "search"
	BatchRenameCommand = "batch-rename"
	TagCommand         = "tag"
	CollectionCommand  = "collection"
	MetadataCommand    = "metadata"

	// We could later make this configurable. But, not needed now.
	spfPromptChar   = ">"
	shellPromptChar = ":"

	successMessagePrefix = "Success"
	failureMessagePrefix = "Error"

	shellModeString = "(Shell Mode)"
	spfModeString   = "(SPF Mode)"

	// Error message string
	tokenizationError    = "Failed during tokenization"
	splitCommandArgError = "split command should not be given arguments"

	// Timeout for command executed for shell substitution
	shellSubTimeout        = 1000 * time.Millisecond
	shellSubTimeoutInTests = 100 * time.Millisecond

	defaultTestCwd = "/"

	PromptMinWidth  = 10
	PromptMinHeight = 3

	defaultTestWidth     = 100
	defaultTestMaxHeight = 100

	// UI dimension constants for prompt modal
	// promptInputPadding is total padding for prompt input fields
	promptInputPadding = 6 // 2 + 1 + 2 + 1 (borders and spacing)

	// expectedArgCount is the expected number of prompt arguments
	expectedArgCount = 2

	operationOnlyArgCount     = 2
	operationValueArgCount    = 3
	operationTwoValueArgCount = 4
)

func modeString(shellMode bool) string {
	if shellMode {
		return shellModeString
	}
	return spfModeString
}

func shellPrompt(shellMode bool) string {
	if shellMode {
		return shellPromptChar
	}
	return spfPromptChar
}

func defaultCommandSlice() []promptCommand {
	return []promptCommand{
		{
			command:     OpenCommand,
			usage:       OpenCommand + " <PATH>",
			description: "Open a new panel at a specified path",
		},
		{
			command:     SplitCommand,
			usage:       SplitCommand,
			description: "Open a new panel at the current file panel's path",
		},
		{
			command:     CdCommand,
			usage:       CdCommand + " <PATH>",
			description: "Change directory of current panel",
		},
		{
			command:     WorkspaceCommand,
			usage:       WorkspaceCommand + " <save|load|delete|list> [NAME]",
			description: "Save, restore, list, or delete named workspaces",
		},
		{
			command:     SearchCommand,
			usage:       SearchCommand + " <save|run|delete|list> [NAME] [QUERY]",
			description: "Save and run named Search Everywhere queries",
		},
		{
			command:     BatchRenameCommand,
			usage:       BatchRenameCommand + " replace FIND VALUE",
			description: "Preview selected filename replacements",
		},
		{
			command:     BatchRenameCommand,
			usage:       BatchRenameCommand + " <prefix|suffix> VALUE",
			description: "Preview adding text to selected names",
		},
		{
			command:     BatchRenameCommand,
			usage:       BatchRenameCommand + " <apply|cancel>",
			description: "Apply or discard the preview",
		},
		{
			command:     TagCommand,
			usage:       TagCommand + " <add|remove|find> NAME; tag list",
			description: "Labels and saved results",
		},
		{
			command:     CollectionCommand,
			usage:       CollectionCommand + " <save|open|delete> NAME; collection list",
			description: "Saved path groups",
		},
		{
			command:     MetadataCommand,
			usage:       MetadataCommand + " <modified|accessed> RFC3339",
			description: "Preview timestamp edit",
		},
		{
			command:     MetadataCommand,
			usage:       MetadataCommand + " permissions OCTAL",
			description: "Preview permissions (0000-0777)",
		},
		{
			command:     MetadataCommand,
			usage:       MetadataCommand + " <apply|restore>",
			description: "Apply preview or restore last edit",
		},
	}
}
