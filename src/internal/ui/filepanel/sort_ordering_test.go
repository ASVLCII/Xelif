package filepanel

import (
	"fmt"
	"testing"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/sortmodel"
)

func TestSortComparatorsAreStrictForEqualKeys(t *testing.T) {
	oldCaseSensitive := common.Config.CaseSensitiveSort
	t.Cleanup(func() { common.Config.CaseSensitiveSort = oldCaseSensitive })
	for _, caseSensitive := range []bool{false, true} {
		common.Config.CaseSensitiveSort = caseSensitive
		for _, kind := range []sortmodel.SortKind{
			sortmodel.SortByName, sortmodel.SortByNatural, sortmodel.SortByType,
			sortmodel.SortByDate, sortmodel.SortBySize,
		} {
			for _, reversed := range []bool{false, true} {
				t.Run(fmt.Sprintf("case_%t/kind_%d/reversed_%t", caseSensitive, kind, reversed), func(t *testing.T) {
					elements := []Element{
						{Name: "File01.TXT", Info: sizeSortFileInfo(10)},
						{Name: "File01.TXT", Info: sizeSortFileInfo(10)},
						{Name: "file01.txt", Info: sizeSortFileInfo(10)},
					}
					less := getOrderingFunc(elements, reversed, kind)
					assertStrictSortOrdering(t, len(elements), less)
				})
			}
		}
	}
}

func assertStrictSortOrdering(t *testing.T, count int, less sliceOrderFunc) {
	t.Helper()
	for i := range count {
		if less(i, i) {
			t.Fatalf("element %d sorts before itself", i)
		}
		for j := range count {
			if less(i, j) && less(j, i) {
				t.Fatalf("elements %d and %d both sort before one another", i, j)
			}
		}
	}
	if less(0, 1) || less(1, 0) {
		t.Fatal("identical sort keys must compare equal")
	}
}
