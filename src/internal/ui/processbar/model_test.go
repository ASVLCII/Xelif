package processbar

import (
	"flag"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/pkg/utils"

	"github.com/yorukot/superfile/src/internal/common"
)

// TODO: This is duplicated in tests of prompt package, internal package too.
// Fix this code duplication

// Initialize the globals we need for testing
func initGlobals() {
	// Updating globals for test is not a good idea and can lead to all sorts of issue
	// When multiple tests depend on same global variable and want different values
	// Since this is config that would likely stay same, maybe this is okay.
	// Also, this is done in main model's test too.
	// We need to find a better way to do this
	err := common.PopulateGlobalConfigs()
	if err != nil {
		fmt.Printf("error while populating config, err : %v", err)
		os.Exit(1)
	}
}

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Verbose() {
		utils.SetRootLoggerToStdout(true)
	} else {
		utils.SetRootLoggerToDiscarded()
	}
	initGlobals()
	m.Run()
}

func TestModelProcessUtils(t *testing.T) {
	m := New()
	p1 := NewProcess("1", "test", OpCopy, 10)
	p2 := NewProcess("2", "test2", OpDelete, 11)

	// ------- Testing AddProcess

	err := m.AddProcess(p1)
	require.NoError(t, err, "Add should succeed without errors")

	err = m.AddProcess(p2)
	require.NoError(t, err, "Add should succeed without errors for second process")

	pRes, ok := m.GetByID(p1.ID)
	require.True(t, ok, "Should be able to get the process we just added")
	assert.Equal(t, p1, pRes, "Should get the correct process value")

	p2Dup := NewProcess("2", "test2_dup", OpCopy, 1)
	err = m.AddProcess(p2Dup)
	var errExp *ProcessAlreadyExistsError
	require.ErrorAs(t, err, &errExp, "Should get ProcessAlreadyExistsError")
	assert.Equal(t, errExp.id, p2Dup.ID, "ID in the error should match with what we sent")

	// ------ Testing AddOrUpdate process
	m.AddOrUpdateProcess(p2Dup)
	pRes, ok = m.GetByID(p2Dup.ID)
	require.True(t, ok)
	assert.Equal(t, p2Dup, pRes, "Should get the correct process value after update")

	p3 := NewProcess("3", "test3", OpExtract, 1)

	// ------ Testing UpdateExisting

	err = m.UpdateExistingProcess(p3)
	var errExpUpdate *NoProcessFoundError
	require.ErrorAs(t, err, &errExpUpdate, "Should get NoProcessFoundError")
	assert.Equal(t, p3.ID, errExpUpdate.id, "ID in the error should match with what we sent")

	assert.True(t, m.HasRunningProcesses())

	// Update all to done
	p1.State = Successful
	p2Dup.Done = p2Dup.Total
	p3.State = Failed
	_ = m.UpdateExistingProcess(p1)
	_ = m.UpdateExistingProcess(p2Dup)
	_ = m.UpdateExistingProcess(p3)

	assert.False(t, m.HasRunningProcesses())
}

func TestModelSetDimenstions(t *testing.T) {
	m := New()

	m.SetDimensions(5, 6)
	assert.Equal(t, 5, m.width, "Correct value should be set")
	assert.Equal(t, 6, m.height, "Correct value should be set")

	m.SetDimensions(minWidth+1, minHeight-1)
	assert.Equal(t, minHeight, m.height, "Min value should be set")
	assert.Equal(t, minWidth+1, m.width, "Given value should be set")
}

func TestCompletedProcessHistoryIsBounded(t *testing.T) {
	m := New()
	running := NewProcess("running", "large-copy", OpCopy, 100)
	require.NoError(t, m.AddProcess(running))

	for i := range maxCompletedProcesses + 5 {
		p := NewProcess(fmt.Sprintf("done-%d", i), "file", OpCopy, 1)
		p.State = Successful
		p.Done = p.Total
		p.DoneTime = time.Unix(int64(i+1), 0)
		m.AddOrUpdateProcess(p)
	}

	assert.Equal(t, maxCompletedProcesses+1, m.cntProcesses())
	_, runningStillPresent := m.GetByID(running.ID)
	assert.True(t, runningStillPresent)
	_, oldestStillPresent := m.GetByID("done-0")
	assert.False(t, oldestStillPresent)
	_, newestStillPresent := m.GetByID(fmt.Sprintf("done-%d", maxCompletedProcesses+4))
	assert.True(t, newestStillPresent)
}

func TestPruningClampsCursorAndRenderIndex(t *testing.T) {
	m := New()
	m.SetDimensions(minWidth, 20)
	for i := range maxCompletedProcesses {
		p := NewProcess(fmt.Sprintf("done-%d", i), "file", OpCopy, 1)
		p.State = Successful
		p.DoneTime = time.Unix(int64(i+1), 0)
		m.AddOrUpdateProcess(p)
	}
	running := NewProcess("running", "file", OpCopy, 1)
	m.AddOrUpdateProcess(running)
	m.cursor = maxCompletedProcesses
	m.renderIndex = m.cursor

	running.State = Successful
	running.DoneTime = time.Unix(1000, 0)
	m.AddOrUpdateProcess(running)

	assert.Less(t, m.cursor, m.cntProcesses())
	assert.LessOrEqual(t, m.renderIndex, m.cursor)
	assert.True(t, m.isValid())
}

func TestPruningKeepsTerminalProcessUntilDoneTimeIsSet(t *testing.T) {
	m := New()
	for i := range maxCompletedProcesses {
		p := NewProcess(fmt.Sprintf("done-%d", i), "file", OpCopy, 1)
		p.State = Successful
		p.DoneTime = time.Unix(int64(i+1), 0)
		m.AddOrUpdateProcess(p)
	}

	failed := NewProcess("failed-with-modal", "file", OpCopy, 1)
	failed.State = Failed
	m.AddOrUpdateProcess(failed)

	_, exists := m.GetByID(failed.ID)
	assert.True(t, exists)
}

func TestConcurrentRequestIDsAreUnique(t *testing.T) {
	m := New()
	const workers = 100
	const perWorker = 1000
	ids := make(chan int, workers*perWorker)
	var wait sync.WaitGroup
	for range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for range perWorker {
				ids <- m.newReqCnt()
			}
		}()
	}
	wait.Wait()
	close(ids)

	seen := make(map[int]struct{}, workers*perWorker)
	for id := range ids {
		seen[id] = struct{}{}
	}
	assert.Len(t, seen, workers*perWorker)
}

func TestConcurrentProcessReadsAndWritesAcrossModelCopies(t *testing.T) {
	m := New()
	copyOfModel := m
	const iterations = 10_000
	start := make(chan struct{})
	var wait sync.WaitGroup

	wait.Add(2)
	go func() {
		defer wait.Done()
		<-start
		for i := range iterations {
			p := Process{ID: "process", State: InOperation, Total: iterations, Done: i}
			copyOfModel.AddOrUpdateProcess(p)
		}
	}()
	go func() {
		defer wait.Done()
		<-start
		for range iterations {
			_ = m.GetProcessesSlice()
			_, _ = m.GetByID("process")
			_ = m.HasRunningProcesses()
		}
	}()

	close(start)
	wait.Wait()
	require.Same(t, m.processesMu, copyOfModel.processesMu)
	assert.Equal(t, 1, m.cntProcesses())
	stored, ok := m.GetByID("process")
	require.True(t, ok)
	assert.Equal(t, iterations-1, stored.Done)
}
