package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pranshuparmar/witr/pkg/model"
)

func TestInitReturnsStartupCommands(t *testing.T) {
	if InitialModel("test").Init() == nil {
		t.Error("Init should return the initial command batch")
	}
}

func TestUpdateContainerDetailMessage(t *testing.T) {
	m := InitialModel("test")
	m, _ = step(t, m, &model.ContainerMatch{Name: "web", ID: "abc123"})
	if m.selectedContainer == nil || m.selectedContainer.Name != "web" {
		t.Errorf("a *ContainerMatch message should populate selectedContainer, got %v", m.selectedContainer)
	}
}

func TestTreeMessagePopulatesViewport(t *testing.T) {
	m := InitialModel("test")
	m.processes = []model.Process{{PID: 42, Command: "x"}}
	m.filterProcesses() // the selected row now carries PID 42

	res := model.Result{
		Process:  model.Process{PID: 42, Command: "x", Cmdline: "x --flag"},
		Ancestry: []model.Process{{PID: 1, Command: "systemd"}, {PID: 42, Command: "x"}},
		Children: []model.Process{{PID: 100, Command: "child"}},
	}
	m, _ = step(t, m, treeMsg(res))

	// handleTree -> updateTreeViewport -> renderTreeContent: ancestry then child.
	if want := []int{1, 42, 100}; !equalInts(m.treePIDs, want) {
		t.Fatalf("treePIDs = %v, want %v", m.treePIDs, want)
	}

	// Navigating the tree in the side pane re-renders it (rerenderTree).
	m.listFocus = focusSide
	before := m.treeCursor
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyUp})
	if m.treeCursor == before {
		t.Errorf("up in the side pane should move the tree cursor from %d", before)
	}
}

func TestTreeShowsExitedParent(t *testing.T) {
	m, _ := step(t, InitialModel("test"), tea.WindowSizeMsg{Width: 160, Height: 40})
	m.processes = []model.Process{{PID: 42, Command: "x"}}
	m.filterProcesses()
	m.table.SetCursor(0)

	res := model.Result{
		Process:  model.Process{PID: 42, Command: "x", ParentExited: true},
		Ancestry: []model.Process{{PID: 1, Command: "systemd"}, {PID: 42, PPID: 1, Command: "x", ParentExited: true}},
	}
	m, _ = step(t, m, treeMsg(res))

	if !strings.Contains(m.treeViewport.View(), "? (original parent exited)") {
		t.Errorf("tree should mark the exited parent:\n%s", m.treeViewport.View())
	}
	// The marker is not a process, so the tree cursor still walks real PIDs.
	if want := []int{1, 42}; !equalInts(m.treePIDs, want) {
		t.Errorf("treePIDs = %v, want %v", m.treePIDs, want)
	}
	// Mouse clicks map screen rows to processes, skipping the marker row.
	if want := []int{0, -1, 1}; !equalInts(m.treeRows, want) {
		t.Errorf("treeRows = %v, want %v", m.treeRows, want)
	}
}

func TestDebounceMessageFetchesSelection(t *testing.T) {
	m := InitialModel("test")
	m.processes = []model.Process{{PID: 42, Command: "x"}}
	m.filterProcesses()
	m.selectionID = 7 // the debounce must carry the live selection id to fire

	_, cmd := step(t, m, debounceMsg{id: 7, pid: 42})
	if cmd == nil {
		t.Error("a debounce matching the current selection should fetch the tree")
	}
}
