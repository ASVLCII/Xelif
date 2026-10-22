package prompt

import (
	"fmt"
	"strings"

	"github.com/yorukot/superfile/src/internal/common"
)

func getPromptAction(shellMode bool, value string, cwdLocation string) (common.ModelAction, error) {
	noAction := common.NoAction{}
	if value == "" {
		return noAction, nil
	}
	if shellMode {
		return common.ShellCommandAction{
			Command: value,
		}, nil
	}

	promptArgs, err := tokenizePromptCommand(value, cwdLocation)
	if err != nil {
		return noAction, invalidCmdError{
			uiMsg:        tokenizationError + " : " + err.Error(),
			wrappedError: fmt.Errorf("error during tokenization : %w", err),
		}
	}

	switch promptArgs[0] {
	case "split":
		if len(promptArgs) != 1 {
			return noAction, invalidCmdError{
				uiMsg: splitCommandArgError,
			}
		}
		return common.SplitPanelAction{}, nil
	case "cd":
		if len(promptArgs) != expectedArgCount {
			return noAction, invalidCmdError{
				uiMsg: fmt.Sprintf("cd command needs exactly one argument, received %d",
					len(promptArgs)-1),
			}
		}
		return common.CDCurrentPanelAction{
			Location: promptArgs[1],
		}, nil
	case "open":
		if len(promptArgs) != expectedArgCount {
			return noAction, invalidCmdError{
				uiMsg: fmt.Sprintf("open command needs exactly one argument, received %d",
					len(promptArgs)-1),
			}
		}
		return common.OpenPanelAction{
			Location: promptArgs[1],
		}, nil
	case WorkspaceCommand:
		return workspacePromptAction(promptArgs)
	case SearchCommand:
		return searchPromptAction(promptArgs)
	case BatchRenameCommand:
		return batchRenamePromptAction(promptArgs)
	case TagCommand:
		return tagPromptAction(promptArgs)
	case CollectionCommand:
		return collectionPromptAction(promptArgs)
	case MetadataCommand:
		return metadataPromptAction(promptArgs)

	default:
		return noAction, invalidCmdError{
			uiMsg: "Invalid spf command : " + promptArgs[0],
		}
	}
}

func metadataPromptAction(promptArgs []string) (common.ModelAction, error) {
	noAction := common.NoAction{}
	if len(promptArgs) < operationOnlyArgCount {
		return noAction, invalidCmdError{uiMsg: "metadata needs modified, accessed, permissions, apply, or restore"}
	}
	operation := common.MetadataOperation(promptArgs[1])
	switch operation {
	case common.MetadataApply, common.MetadataRestore:
		if len(promptArgs) != operationOnlyArgCount {
			return noAction, invalidCmdError{uiMsg: fmt.Sprintf("metadata %s accepts no arguments", operation)}
		}
		return common.MetadataAction{Operation: operation}, nil
	case common.MetadataModified, common.MetadataAccessed, common.MetadataPermissions:
		if len(promptArgs) != operationValueArgCount {
			return noAction, invalidCmdError{uiMsg: fmt.Sprintf("metadata %s needs exactly one value", operation)}
		}
		return common.MetadataAction{Operation: operation, Value: promptArgs[2]}, nil
	default:
		return noAction, invalidCmdError{uiMsg: "unknown metadata operation: " + promptArgs[1]}
	}
}

func tagPromptAction(promptArgs []string) (common.ModelAction, error) {
	return catalogPromptAction(promptArgs, catalogPromptSpec{
		command: "tag", missingMessage: "tag needs add, remove, list, or find", valueName: "tag name",
		listOperation: common.TagList,
		operations: map[string]common.CatalogOperation{
			"add": common.TagAdd, "remove": common.TagRemove, "list": common.TagList, "find": common.TagFind,
		},
	})
}

func collectionPromptAction(promptArgs []string) (common.ModelAction, error) {
	return catalogPromptAction(promptArgs, catalogPromptSpec{
		command: "collection", missingMessage: "collection needs save, open, list, or delete", valueName: "name",
		listOperation: common.CollectionList,
		operations: map[string]common.CatalogOperation{
			"save": common.CollectionSave, "open": common.CollectionOpen,
			"list": common.CollectionList, "delete": common.CollectionDelete,
		},
	})
}

type catalogPromptSpec struct {
	command        string
	missingMessage string
	valueName      string
	listOperation  common.CatalogOperation
	operations     map[string]common.CatalogOperation
}

func catalogPromptAction(promptArgs []string, spec catalogPromptSpec) (common.ModelAction, error) {
	noAction := common.NoAction{}
	if len(promptArgs) < operationOnlyArgCount {
		return noAction, invalidCmdError{uiMsg: spec.missingMessage}
	}
	operationName := promptArgs[1]
	operation, ok := spec.operations[operationName]
	if !ok {
		return noAction, invalidCmdError{uiMsg: "unknown " + spec.command + " operation: " + operationName}
	}
	if operation == spec.listOperation {
		if len(promptArgs) != operationOnlyArgCount {
			return noAction, invalidCmdError{uiMsg: spec.command + " list accepts no arguments"}
		}
		return common.CatalogAction{Operation: operation}, nil
	}
	if len(promptArgs) != operationValueArgCount {
		return noAction, invalidCmdError{
			uiMsg: spec.command + " " + operationName + " needs exactly one " + spec.valueName,
		}
	}
	return common.CatalogAction{Operation: operation, Name: promptArgs[2]}, nil
}

func batchRenamePromptAction(promptArgs []string) (common.ModelAction, error) {
	noAction := common.NoAction{}
	if len(promptArgs) < operationOnlyArgCount {
		return noAction, invalidCmdError{uiMsg: "batch-rename needs replace, prefix, suffix, apply, or cancel"}
	}
	operation := common.BatchRenameOperation(promptArgs[1])
	switch operation {
	case common.BatchRenameReplace:
		if len(promptArgs) != operationTwoValueArgCount {
			return noAction, invalidCmdError{uiMsg: "batch-rename replace needs FIND and REPLACEMENT"}
		}
		return common.BatchRenameAction{
			Operation: operation, Find: promptArgs[2], Replacement: promptArgs[3],
		}, nil
	case common.BatchRenamePrefix, common.BatchRenameSuffix:
		if len(promptArgs) != operationValueArgCount {
			return noAction, invalidCmdError{uiMsg: fmt.Sprintf("batch-rename %s needs one value", operation)}
		}
		return common.BatchRenameAction{Operation: operation, Replacement: promptArgs[2]}, nil
	case common.BatchRenameApply, common.BatchRenameCancel:
		if len(promptArgs) != operationOnlyArgCount {
			return noAction, invalidCmdError{uiMsg: fmt.Sprintf("batch-rename %s accepts no arguments", operation)}
		}
		return common.BatchRenameAction{Operation: operation}, nil
	default:
		return noAction, invalidCmdError{uiMsg: "unknown batch-rename operation: " + promptArgs[1]}
	}
}

func searchPromptAction(promptArgs []string) (common.ModelAction, error) {
	noAction := common.NoAction{}
	if len(promptArgs) < operationOnlyArgCount {
		return noAction, invalidCmdError{uiMsg: "search needs an operation: save, run, delete, or list"}
	}
	operation := common.SearchOperation(promptArgs[1])
	switch operation {
	case common.SearchList:
		if len(promptArgs) != operationOnlyArgCount {
			return noAction, invalidCmdError{uiMsg: "search list does not accept arguments"}
		}
		return common.SearchAction{Operation: operation}, nil
	case common.SearchRun, common.SearchDelete:
		if len(promptArgs) != operationValueArgCount {
			return noAction, invalidCmdError{uiMsg: fmt.Sprintf("search %s needs exactly one name", operation)}
		}
		return common.SearchAction{Operation: operation, Name: promptArgs[2]}, nil
	case common.SearchSave:
		if len(promptArgs) != operationTwoValueArgCount {
			return noAction, invalidCmdError{
				uiMsg: "search save needs a name and one quoted query",
			}
		}
		return common.SearchAction{Operation: operation, Name: promptArgs[2], Query: promptArgs[3]}, nil
	default:
		return noAction, invalidCmdError{uiMsg: "unknown search operation: " + promptArgs[1]}
	}
}

func workspacePromptAction(promptArgs []string) (common.ModelAction, error) {
	noAction := common.NoAction{}
	if len(promptArgs) < operationOnlyArgCount {
		return noAction, invalidCmdError{uiMsg: "workspace needs an operation: save, load, delete, or list"}
	}
	operation := common.WorkspaceOperation(promptArgs[1])
	switch operation {
	case common.WorkspaceList:
		if len(promptArgs) != operationOnlyArgCount {
			return noAction, invalidCmdError{uiMsg: "workspace list does not accept a name"}
		}
		return common.WorkspaceAction{Operation: operation}, nil
	case common.WorkspaceSave, common.WorkspaceLoad, common.WorkspaceDelete:
		if len(promptArgs) != operationValueArgCount {
			return noAction, invalidCmdError{
				uiMsg: fmt.Sprintf("workspace %s needs exactly one name", operation),
			}
		}
		return common.WorkspaceAction{Operation: operation, Name: promptArgs[2]}, nil
	default:
		return noAction, invalidCmdError{uiMsg: "unknown workspace operation: " + promptArgs[1]}
	}
}

// Only allocates memory proportional to first token's size
// Only works for space right now. Does not splits command based on
// \n or \t , etc
func getFirstToken(command string) string {
	command = strings.TrimSpace(command)
	spaceIndex := strings.IndexByte(command, ' ')
	if spaceIndex == -1 {
		return command
	}
	return command[:spaceIndex]
}
