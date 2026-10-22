package internal

import "github.com/yorukot/superfile/src/internal/common"

// Only commands that deliberately open a results surface transfer focus. A
// pre-existing preview must not override keep-open for ordinary command output.
func promptOpensResults(action common.ModelAction) bool {
	switch action := action.(type) {
	case common.BatchRenameAction:
		return action.Operation == common.BatchRenameReplace || action.Operation == common.BatchRenamePrefix ||
			action.Operation == common.BatchRenameSuffix || action.Operation == common.BatchRenameApply
	case common.MetadataAction:
		return action.Operation == common.MetadataModified || action.Operation == common.MetadataAccessed ||
			action.Operation == common.MetadataPermissions || action.Operation == common.MetadataApply ||
			action.Operation == common.MetadataRestore
	case common.CatalogAction:
		return action.Operation == common.TagFind || action.Operation == common.CollectionOpen
	case common.SearchAction:
		return action.Operation == common.SearchRun
	default:
		return false
	}
}
