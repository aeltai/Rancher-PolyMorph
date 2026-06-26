package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aeltai/rancher-polymorph/internal/ascii"
	"github.com/aeltai/rancher-polymorph/internal/backup"
	"github.com/aeltai/rancher-polymorph/internal/config"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type screen int

const (
	screenSplash screen = iota
	screenMenu
	screenInput
	screenLoading
	screenInspectView
	screenMode
	screenCluster
	screenTreePreview
	screenGhosts
	screenOutput
	screenConfirm
	screenRunning
	screenDone
	screenRestoreInput
	screenRestoreRunning
	screenSourceSelect
	screenS3KeyList
	screenPostSanitize
)

type pendingAfterLoad int

const (
	loadToInspect pendingAfterLoad = iota
	loadToTreePreview
	loadToOutput
	loadToS3List
	loadToS3Pull
)

type flow int

const (
	flowSanitize flow = iota
	flowInspectOnly
	flowRestore
	flowMigrate
	flowS3Pull
)

type menuItem struct {
	title, desc string
}

func (i menuItem) Title() string       { return i.title }
func (i menuItem) Description() string { return i.desc }
func (i menuItem) FilterValue() string { return i.title }

type previewDoneMsg struct {
	res *backup.PreviewResult
	err error
}

type inspectDoneMsg struct {
	res  *backup.InspectResult
	tree *backup.PreviewResult
	err  error
}

type sanitizeDoneMsg struct {
	res *backup.Result
	err error
}

type progressMsg struct {
	current, total int
}

type splashTickMsg time.Time

type model struct {
	width, height int
	screen        screen
	flow          flow
	err           string
	cfg           config.Config
	splashAt      time.Time
	animations    bool

	menu           list.Model
	modeList       list.Model
	sourceList     list.Model
	s3KeyList      list.Model
	postActionList list.Model
	input          textinput.Model
	outputInput    textinput.Model
	reportInput    textinput.Model
	outputFocus    int // 0=output, 1=report

	spinner     spinner.Model
	inspect     *backup.InspectResult
	inspectTree *backup.PreviewResult
	result      *backup.Result
	backupPath  string

	keepRKE1Only bool
	fast         bool
	autoOrphans  bool

	clusterCursor   int
	clusterSelected map[string]bool
	preview         *backup.PreviewResult
	previewAfter    *backup.PreviewResult
	treeViewport    viewport.Model
	treeExpanded    map[string]bool
	treeGroupCursor int
	treeTab         treeTab
	afterLoad       pendingAfterLoad

	progressCur   int
	progressTotal int
	doneLines     []string
	sanitizeCh    chan tea.Msg
	restoreLocal  string
	restoreStatus string
	restoreCh     chan tea.Msg
}

func newModel(cfg config.Config) model {
	menuItems := []list.Item{
		menuItem{"Full migration", "S3 or local → sanitize → restore on target cluster"},
		menuItem{"Sanitize backup", "Local file — inspect, filter clusters, write tarball"},
		menuItem{"Pull from S3", "Download backup .tar.gz to local backups/"},
		menuItem{"Inspect only", "Read-only inventory tree"},
		menuItem{"Restore to cluster", "kubectl cp + Restore CR + wait for Ready"},
		menuItem{"Quit", "Exit rancher-polymorph"},
	}
	menu := list.New(menuItems, list.NewDefaultDelegate(), 0, 0)
	menu.Title = "What would you like to do?"
	menu.SetShowStatusBar(false)
	menu.SetFilteringEnabled(false)
	menu.SetShowHelp(true)

	modeItems := []list.Item{
		menuItem{"Keep selected clusters", "Pick one or more downstream clusters to retain"},
		menuItem{"Keep all RKE1", "Remove imported / RKE2 clusters only"},
	}
	modeList := list.New(modeItems, list.NewDefaultDelegate(), 0, 0)
	modeList.Title = "Cluster retention mode"
	modeList.SetShowStatusBar(false)
	modeList.SetFilteringEnabled(false)

	ti := textinput.New()
	ti.Placeholder = "/path/to/rancher-backup.tar.gz"
	ti.CharLimit = 512
	ti.Width = 60
	ti.Focus()

	out := textinput.New()
	out.Placeholder = "sanitized output .tar.gz"
	out.CharLimit = 512
	out.Width = 60

	rep := textinput.New()
	rep.Placeholder = "optional report .txt"
	rep.CharLimit = 512
	rep.Width = 60

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	startScreen := screenMenu
	if cfg.UI.Animations {
		startScreen = screenSplash
	}

	return model{
		screen:          startScreen,
		splashAt:        time.Now(),
		animations:      cfg.UI.Animations,
		cfg:             cfg,
		menu:            menu,
		modeList:        modeList,
		sourceList:      newSourceList(80, 24),
		s3KeyList:       list.New(nil, list.NewDefaultDelegate(), 60, 10),
		postActionList:  newPostSanitizeList(80, 24),
		input:           ti,
		outputInput:     out,
		reportInput:     rep,
		spinner:         sp,
		autoOrphans:     cfg.AutoOrphansEnabled(),
		fast:            cfg.Defaults.Fast,
		clusterSelected: make(map[string]bool),
		treeExpanded:    make(map[string]bool),
		treeViewport:    newTreeViewport(80, 24),
	}
}

func (m model) Init() tea.Cmd {
	if m.screen == screenSplash {
		return splashTickCmd()
	}
	return nil
}

func splashTickCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg { return splashTickMsg(t) })
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.menu.SetSize(msg.Width-4, min(14, msg.Height-8))
		m.modeList.SetSize(msg.Width-4, min(8, msg.Height-8))
		m.sourceList.SetSize(msg.Width-4, min(8, msg.Height-8))
		m.s3KeyList.SetSize(msg.Width-4, min(16, msg.Height-10))
		m.postActionList.SetSize(msg.Width-4, min(8, msg.Height-8))
		m.treeViewport.Width = max(20, msg.Width-4)
		m.treeViewport.Height = max(8, msg.Height-16)
		return m, nil

	case splashTickMsg:
		if m.screen != screenSplash {
			return m, nil
		}
		if time.Since(m.splashAt) > 2800*time.Millisecond {
			m.screen = screenMenu
			return m, nil
		}
		return m, splashTickCmd()

	case restoreProgressMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
			if m.flow == flowMigrate && m.result != nil {
				m.screen = screenPostSanitize
			} else {
				m.screen = screenRestoreInput
			}
			return m, nil
		}
		m.restoreStatus = msg.status
		if msg.backupName != "" {
			m.restoreLocal = msg.backupName
		}
		if msg.done {
			m.restoreCh = nil
			m.screen = screenDone
			return m, nil
		}
		return m, tea.Batch(m.spinner.Tick, waitSanitizeMsg(m.restoreCh))

	case s3ListDoneMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
			m.screen = screenMenu
			return m, nil
		}
		m.s3KeyList = newS3KeyList(msg.keys, m.width, m.height)
		m.screen = screenS3KeyList
		return m, nil

	case s3PullDoneMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
			m.screen = screenS3KeyList
			return m, nil
		}
		m.backupPath = msg.localPath
		m.doneLines = []string{fmt.Sprintf("Downloaded: %s", msg.localPath)}
		if m.flow == flowS3Pull {
			m.screen = screenDone
			return m, nil
		}
		return m.beginSanitizeAfterPull(msg.localPath)

	case previewDoneMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
			if m.afterLoad == loadToTreePreview {
				m.screen = screenCluster
			} else {
				m.screen = screenGhosts
			}
			return m, nil
		}
		m.preview = msg.res
		m.treeGroupCursor = 0
		for k := range m.treeExpanded {
			delete(m.treeExpanded, k)
		}
		refreshTreeViewport(&m.treeViewport, m.preview, m.treeExpanded, m.width, m.treeGroupCursor)
		switch m.afterLoad {
		case loadToOutput:
			m.screen = screenOutput
			m.outputFocus = 0
			m.outputInput.Focus()
			m.reportInput.Blur()
			return m, textinput.Blink
		default:
			m.screen = screenTreePreview
		}
		return m, nil

	case inspectDoneMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
			m.screen = screenInput
			return m, nil
		}
		m.inspect = msg.res
		m.inspectTree = msg.tree
		m.treeGroupCursor = 0
		for k := range m.treeExpanded {
			delete(m.treeExpanded, k)
		}
		if m.inspectTree != nil {
			refreshTreeViewport(&m.treeViewport, m.inspectTree, m.treeExpanded, m.width, m.treeGroupCursor)
		}
		m.err = ""
		if m.flow == flowInspectOnly {
			m.screen = screenInspectView
			return m, nil
		}
		m.screen = screenInspectView
		return m, nil

	case progressMsg:
		m.progressCur = msg.current
		m.progressTotal = msg.total
		if m.sanitizeCh != nil {
			return m, waitSanitizeMsg(m.sanitizeCh)
		}
		return m, nil

	case sanitizeDoneMsg:
		m.sanitizeCh = nil
		if msg.err != nil {
			m.err = msg.err.Error()
			m.screen = screenConfirm
			return m, nil
		}
		m.result = msg.res
		m.doneLines = buildDoneLines(msg.res)
		m.previewAfter = backup.PreviewFromResult(msg.res)
		m.treeTab = treeTabAfter
		m.treeGroupCursor = 0
		refreshTreeViewport(&m.treeViewport, m.activePreview(), m.treeExpanded, m.width, m.treeGroupCursor)
		if m.flow == flowMigrate {
			m.postActionList = newPostSanitizeList(m.width, m.height)
			m.screen = screenPostSanitize
			return m, nil
		}
		m.screen = screenDone
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if m.screen == screenLoading || m.screen == screenRunning || m.screen == screenRestoreRunning {
			return m, tea.Batch(cmd, m.spinner.Tick)
		}
		return m, cmd

	case tea.KeyMsg:
		if m.screen == screenLoading || m.screen == screenRunning || m.screen == screenRestoreRunning {
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			return m, nil
		}

		if m.screen == screenSplash {
			if msg.String() == "enter" || msg.String() == " " || msg.String() == "q" {
				m.screen = screenMenu
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c", "q":
			if m.screen == screenMenu {
				return m, tea.Quit
			}
			if m.screen == screenDone || m.screen == screenInspectView && m.flow == flowInspectOnly {
				return m, tea.Quit
			}
		}

		switch m.screen {
		case screenMenu:
			return m.updateMenu(msg)
		case screenInput:
			return m.updateInput(msg)
		case screenRestoreInput:
			return m.updateRestoreInput(msg)
		case screenSourceSelect:
			return m.updateSourceSelect(msg)
		case screenS3KeyList:
			return m.updateS3KeyList(msg)
		case screenPostSanitize:
			return m.updatePostSanitize(msg)
		case screenInspectView:
			return m.updateInspectView(msg)
		case screenMode:
			return m.updateMode(msg)
		case screenCluster:
			return m.updateCluster(msg)
		case screenTreePreview:
			return m.updateTreePreview(msg)
		case screenGhosts:
			return m.updateGhosts(msg)
		case screenOutput:
			return m.updateOutput(msg)
		case screenConfirm:
			return m.updateConfirm(msg)
		case screenDone:
			return m.updateDone(msg)
		}
	}

	return m, nil
}

func (m model) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() != "enter" {
		var cmd tea.Cmd
		m.menu, cmd = m.menu.Update(msg)
		return m, cmd
	}
	switch m.menu.Index() {
	case 0:
		m.flow = flowMigrate
		m.err = ""
		m.screen = screenSourceSelect
	case 1:
		m.flow = flowSanitize
		m.screen = screenInput
		m.input.SetValue(m.backupPath)
		m.input.Placeholder = "/path/to/rancher-backup.tar.gz"
		m.input.Focus()
	case 2:
		m.flow = flowS3Pull
		m.err = ""
		if !s3Configured(m.cfg) {
			m.err = "s3.bucket not set — configure rancher-polymorph.yaml"
			return m, nil
		}
		m.screen = screenLoading
		m.afterLoad = loadToS3List
		return m, tea.Batch(m.spinner.Tick, runS3List(m.cfg))
	case 3:
		m.flow = flowInspectOnly
		m.screen = screenInput
		m.input.SetValue(m.backupPath)
		m.input.Focus()
	case 4:
		m.flow = flowRestore
		m.screen = screenRestoreInput
		m.input.SetValue(m.restoreLocal)
		m.input.Placeholder = "/path/to/sanitized-backup.tar.gz"
		m.input.Focus()
	case 5:
		return m, tea.Quit
	}
	return m, textinput.Blink
}

func (m model) updateRestoreInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.err = ""
		return m, nil
	case "enter":
		path := strings.TrimSpace(m.input.Value())
		if path == "" {
			m.err = "sanitized backup path is required"
			return m, nil
		}
		if _, err := os.Stat(path); err != nil {
			m.err = fmt.Sprintf("cannot read file: %v", err)
			return m, nil
		}
		if m.cfg.Restore.Kubeconfig == "" {
			m.err = "restore.kubeconfig not set — run: rancher-polymorph config init"
			return m, nil
		}
		m.restoreLocal = path
		m.screen = screenRestoreRunning
		m.restoreStatus = "Starting restore…"
		ch := make(chan tea.Msg, 12)
		m.restoreCh = ch
		go restoreGoroutine(m.cfg, path, ch)
		return m, tea.Batch(m.spinner.Tick, waitSanitizeMsg(ch))
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.err = ""
		return m, nil
	case "enter":
		path := strings.TrimSpace(m.input.Value())
		if path == "" {
			m.err = "backup path is required"
			return m, nil
		}
		if _, err := os.Stat(path); err != nil {
			m.err = fmt.Sprintf("cannot read backup: %v", err)
			return m, nil
		}
		m.backupPath = path
		m.afterLoad = loadToInspect
		m.screen = screenLoading
		m.err = ""
		return m, tea.Batch(m.spinner.Tick, runInspect(path))
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m model) updateInspectView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		return m, nil
	case "enter":
		if m.flow == flowInspectOnly {
			return m, tea.Quit
		}
		m.screen = screenMode
		return m, nil
	default:
		return m.updateTreeKeys(msg, m.inspectTree)
	}
}

func (m model) updateTreeKeys(msg tea.KeyMsg, tree *backup.PreviewResult) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if tree != nil {
			toggleGroupExpand(m.treeExpanded, tree, m.treeGroupCursor)
			refreshTreeViewport(&m.treeViewport, tree, m.treeExpanded, m.width, m.treeGroupCursor)
		}
		return m, nil
	case "j":
		if tree != nil && m.treeGroupCursor < len(tree.Groups)-1 {
			m.treeGroupCursor++
			scrollTreeToGroup(&m.treeViewport, tree, m.treeExpanded, m.treeGroupCursor, m.width)
			refreshTreeViewport(&m.treeViewport, tree, m.treeExpanded, m.width, m.treeGroupCursor)
		}
		return m, nil
	case "k":
		if m.treeGroupCursor > 0 {
			m.treeGroupCursor--
			scrollTreeToGroup(&m.treeViewport, tree, m.treeExpanded, m.treeGroupCursor, m.width)
			refreshTreeViewport(&m.treeViewport, tree, m.treeExpanded, m.width, m.treeGroupCursor)
		}
		return m, nil
	default:
		var cmd tea.Cmd
		m.treeViewport, cmd = m.treeViewport.Update(msg)
		return m, cmd
	}
}

func (m model) updateMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.screen = screenInspectView
		return m, nil
	}
	if msg.String() != "enter" {
		var cmd tea.Cmd
		m.modeList, cmd = m.modeList.Update(msg)
		return m, cmd
	}
	m.keepRKE1Only = m.modeList.Index() == 1
	if m.keepRKE1Only {
		m.afterLoad = loadToTreePreview
		m.screen = screenLoading
		return m, tea.Batch(m.spinner.Tick, runPreview(m.previewOptionsForRKE1()))
	}
	m.initClusterSelection()
	m.screen = screenCluster
	return m, nil
}

func (m *model) initClusterSelection() {
	m.clusterSelected = make(map[string]bool)
	m.clusterCursor = 0
	ids := selectableClusterIDs(m.inspect.Clusters)
	if len(ids) == 1 {
		m.clusterSelected[ids[0]] = true
	}
}

func (m model) updateCluster(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	ids := selectableClusterIDs(m.inspect.Clusters)
	if len(ids) == 0 {
		m.err = "no downstream clusters in backup"
		return m, nil
	}
	switch msg.String() {
	case "esc":
		m.screen = screenMode
		return m, nil
	case "up", "k":
		if m.clusterCursor > 0 {
			m.clusterCursor--
		}
		return m, nil
	case "down", "j":
		if m.clusterCursor < len(ids)-1 {
			m.clusterCursor++
		}
		return m, nil
	case " ":
		cid := ids[m.clusterCursor]
		m.clusterSelected[cid] = !m.clusterSelected[cid]
		return m, nil
	case "a":
		for _, cid := range ids {
			m.clusterSelected[cid] = true
		}
		return m, nil
	case "n":
		for _, cid := range ids {
			m.clusterSelected[cid] = false
		}
		return m, nil
	case "enter":
		if !m.hasClusterSelection() {
			m.err = "select at least one cluster to keep"
			return m, nil
		}
		m.err = ""
		m.afterLoad = loadToTreePreview
		m.screen = screenLoading
		return m, tea.Batch(m.spinner.Tick, runPreview(m.previewOptions()))
	}
	return m, nil
}

func (m model) hasClusterSelection() bool {
	for _, on := range m.clusterSelected {
		if on {
			return true
		}
	}
	return false
}

func (m model) updateTreePreview(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if m.keepRKE1Only {
			m.screen = screenMode
		} else {
			m.screen = screenCluster
		}
		return m, nil
	case "tab":
		if m.previewAfter != nil {
			if m.treeTab == treeTabBefore {
				m.treeTab = treeTabAfter
			} else {
				m.treeTab = treeTabBefore
			}
			refreshTreeViewport(&m.treeViewport, m.activePreview(), m.treeExpanded, m.width, m.treeGroupCursor)
		}
		return m, nil
	case "enter":
		p := m.activePreview()
		if p != nil {
			toggleGroupExpand(m.treeExpanded, p, m.treeGroupCursor)
			refreshTreeViewport(&m.treeViewport, p, m.treeExpanded, m.width, m.treeGroupCursor)
		}
		return m, nil
	case "c":
		m.screen = screenGhosts
		return m, nil
	case "j", "k":
		return m.updateTreeKeys(msg, m.activePreview())
	default:
		var cmd tea.Cmd
		m.treeViewport, cmd = m.treeViewport.Update(msg)
		return m, cmd
	}
}

func (m model) updateDone(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab":
		if m.previewAfter != nil && m.preview != nil {
			if m.treeTab == treeTabBefore {
				m.treeTab = treeTabAfter
			} else {
				m.treeTab = treeTabBefore
			}
			refreshTreeViewport(&m.treeViewport, m.activePreview(), m.treeExpanded, m.width, m.treeGroupCursor)
		}
		return m, nil
	case "enter":
		if m.flow == flowS3Pull && m.backupPath != "" {
			m.flow = flowSanitize
			return m.beginSanitizeAfterPull(m.backupPath)
		}
		if m.flow == flowRestore {
			m.screen = screenMenu
			m.flow = flowSanitize
			m.err = ""
			return m, nil
		}
		if m.previewAfter != nil || m.preview != nil {
			return m.updateTreeKeys(msg, m.activePreview())
		}
		return m, tea.Quit
	case "j", "k":
		if m.flow != flowRestore && (m.previewAfter != nil || m.preview != nil) {
			return m.updateTreeKeys(msg, m.activePreview())
		}
		return m, nil
	case "q":
		if m.flow == flowRestore {
			m.screen = screenMenu
			m.flow = flowSanitize
			m.err = ""
			return m, nil
		}
		return m, tea.Quit
	case "esc":
		if m.flow == flowS3Pull {
			m.screen = screenMenu
			m.err = ""
			return m, nil
		}
		return m, nil
	default:
		if m.flow != flowRestore {
			var cmd tea.Cmd
			m.treeViewport, cmd = m.treeViewport.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m model) activePreview() *backup.PreviewResult {
	if m.screen == screenInspectView {
		return m.inspectTree
	}
	if m.treeTab == treeTabAfter && m.previewAfter != nil {
		return m.previewAfter
	}
	return m.preview
}

func (m model) updateGhosts(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenTreePreview
	case "a":
		m.autoOrphans = !m.autoOrphans
	case "enter":
		m.applyOutputDefaults()
		if m.preview == nil {
			m.afterLoad = loadToOutput
			m.screen = screenLoading
			return m, tea.Batch(m.spinner.Tick, runPreview(m.previewOptions()))
		}
		m.screen = screenOutput
		m.outputFocus = 0
		m.outputInput.Focus()
		m.reportInput.Blur()
		return m, textinput.Blink
	}
	return m, nil
}

func (m model) updateOutput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenGhosts
		return m, nil
	case "tab", "shift+tab":
		m.outputFocus = 1 - m.outputFocus
		if m.outputFocus == 0 {
			m.outputInput.Focus()
			m.reportInput.Blur()
		} else {
			m.reportInput.Focus()
			m.outputInput.Blur()
		}
		return m, textinput.Blink
	case "f":
		m.fast = !m.fast
		return m, nil
	case "enter":
		if strings.TrimSpace(m.outputInput.Value()) == "" {
			m.err = "output path is required"
			return m, nil
		}
		m.err = ""
		m.screen = screenConfirm
		return m, nil
	}
	var cmd tea.Cmd
	if m.outputFocus == 0 {
		m.outputInput, cmd = m.outputInput.Update(msg)
	} else {
		m.reportInput, cmd = m.reportInput.Update(msg)
	}
	return m, cmd
}

func (m model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenOutput
		return m, nil
	case "enter":
		opts := m.buildSanitizeOptions()
		m.screen = screenRunning
		m.progressCur = 0
		m.progressTotal = 1
		ch := make(chan tea.Msg, 4)
		m.sanitizeCh = ch
		var lastProgress time.Time
		opts.ProgressFn = func(cur, total int) {
			// Throttle UI updates; never block sanitize on a full channel (deadlock).
			if cur < total && time.Since(lastProgress) < 80*time.Millisecond {
				return
			}
			lastProgress = time.Now()
			msg := progressMsg{current: cur, total: total}
			if cur >= total {
				ch <- msg
				return
			}
			select {
			case ch <- msg:
			default:
			}
		}
		go func() {
			res, err := backup.Sanitize(opts)
			ch <- sanitizeDoneMsg{res: res, err: err}
		}()
		return m, tea.Batch(m.spinner.Tick, waitSanitizeMsg(ch))
	case "n":
		m.screen = screenOutput
		return m, nil
	}
	return m, nil
}

func (m model) applyOutputDefaults() {
	in := m.backupPath
	dir := m.cfg.Defaults.OutputDir
	if dir == "" {
		dir = filepath.Dir(in)
	}
	base := strings.TrimSuffix(filepath.Base(in), ".tar.gz")
	m.outputInput.SetValue(filepath.Join(dir, base+"-sanitized.tar.gz"))
	repDir := m.cfg.Defaults.ReportDir
	if repDir == "" {
		repDir = dir
	}
	m.reportInput.SetValue(filepath.Join(repDir, base+"-sanitize-report.txt"))
}

func (m model) selectedClusterIDs() []string {
	var ids []string
	for cid, on := range m.clusterSelected {
		if on {
			ids = append(ids, cid)
		}
	}
	sort.Strings(ids)
	return ids
}

func (m model) previewOptions() backup.Options {
	opts := m.buildSanitizeOptions()
	opts.InspectOnly = true
	return opts
}

func (m model) previewOptionsForRKE1() backup.Options {
	opts := backup.Options{
		Input:         m.backupPath,
		KeepRKE1Only:  true,
		NoAutoOrphans: !m.autoOrphans,
		InspectOnly:   true,
	}
	return opts
}

func (m model) buildSanitizeOptions() backup.Options {
	opts := backup.Options{
		Input:         m.backupPath,
		Output:        strings.TrimSpace(m.outputInput.Value()),
		Report:        strings.TrimSpace(m.reportInput.Value()),
		KeepRKE1Only:  m.keepRKE1Only,
		NoAutoOrphans: !m.autoOrphans,
		Fast:          m.fast,
		Quiet:         true,
	}
	if !m.keepRKE1Only {
		opts.KeepClusters = m.selectedClusterIDs()
	}
	return opts
}

func runPreview(opts backup.Options) tea.Cmd {
	return func() tea.Msg {
		res, err := backup.PreviewSanitize(opts)
		return previewDoneMsg{res: res, err: err}
	}
}

func (m model) View() string {
	if m.screen == screenSplash {
		return lipgloss.NewStyle().
			Width(m.width).
			Render(ascii.SplashFrame(time.Now(), m.width, m.splashAt) + "\n" + hintStyle.Render("press enter to continue"))
	}

	var b strings.Builder
	b.WriteString(subtitleStyle.Render(ascii.CompactHeader()))
	b.WriteString("\n\n")

	if m.err != "" {
		b.WriteString(errStyle.Render("Error: " + m.err))
		b.WriteString("\n\n")
	}

	switch m.screen {
	case screenMenu:
		b.WriteString(m.menu.View())
	case screenInput:
		b.WriteString(boxStyle.Render("Backup tarball path"))
		b.WriteString("\n")
		b.WriteString(m.input.View())
		b.WriteString("\n\n")
		b.WriteString(hintStyle.Render("enter submit · esc menu"))
	case screenRestoreInput:
		b.WriteString(boxStyle.Render("Restore sanitized backup"))
		b.WriteString("\n")
		b.WriteString(m.input.View())
		b.WriteString("\n\n")
		kc := m.cfg.Restore.Kubeconfig
		if kc == "" {
			kc = "(not set — run: rancher-polymorph config init)"
		}
		b.WriteString(fmt.Sprintf("Kubeconfig: %s\n", kc))
		b.WriteString(fmt.Sprintf("Operator:   %s  label: %s\n", m.cfg.Restore.OperatorNamespace, m.cfg.Restore.BackupPodLabel))
		b.WriteString(hintStyle.Render("enter start restore (copy + apply + watch) · esc menu"))
	case screenSourceSelect:
		b.WriteString(m.sourceList.View())
	case screenS3KeyList:
		b.WriteString(renderS3KeyList(m.cfg))
		b.WriteString(m.s3KeyList.View())
		b.WriteString("\n")
		b.WriteString(hintStyle.Render("enter download · esc menu"))
	case screenPostSanitize:
		for _, line := range m.doneLines {
			b.WriteString(line)
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(m.postActionList.View())
		b.WriteString("\n")
		b.WriteString(hintStyle.Render("enter select · esc menu"))
	case screenLoading:
		b.WriteString(fmt.Sprintf("%s\n\n", m.spinner.View()))
		b.WriteString(ascii.IndeterminateLoader(m.width, time.Now()))
		b.WriteString("\n\n")
		if m.inspect == nil {
			switch m.afterLoad {
			case loadToS3List:
				b.WriteString(subtitleStyle.Render("Listing S3 backups…"))
			case loadToS3Pull:
				b.WriteString(subtitleStyle.Render("Downloading from S3…"))
			default:
				b.WriteString(subtitleStyle.Render("Inspecting backup…"))
			}
		} else {
			b.WriteString(subtitleStyle.Render("Building keep/drop tree…"))
		}
		b.WriteString("\n")
		b.WriteString(subtitleStyle.Render(m.backupPath))
	case screenInspectView:
		b.WriteString(renderInspectBrief(m.inspect))
		b.WriteString("\n")
		if m.inspectTree != nil {
			b.WriteString(renderTreeHeader(m.inspectTree, treeTabBefore, false, m.treeGroupCursor))
			b.WriteString("\n")
			b.WriteString(m.treeViewport.View())
			b.WriteString("\n")
			b.WriteString(hintStyle.Render("enter expand · j/k group · ↑↓ scroll · esc menu"))
		}
		if m.flow == flowSanitize {
			b.WriteString("\n")
			b.WriteString(hintStyle.Render("enter continue to sanitize"))
		} else {
			b.WriteString("\n")
			b.WriteString(hintStyle.Render("enter quit"))
		}
	case screenMode:
		b.WriteString(m.modeList.View())
	case screenCluster:
		b.WriteString(renderClusterPicker(m.inspect.Clusters, m.clusterSelected, m.clusterCursor, m.width))
	case screenTreePreview:
		showTabs := m.previewAfter != nil
		b.WriteString(renderTreeHeader(m.activePreview(), m.treeTab, showTabs, m.treeGroupCursor))
		b.WriteString("\n")
		b.WriteString(m.treeViewport.View())
		b.WriteString("\n")
		b.WriteString(hintStyle.Render("c continue · enter expand group · j/k group · ↑↓ scroll · esc back"))
	case screenGhosts:
		b.WriteString(renderGhosts(m.inspect, m.autoOrphans))
		b.WriteString("\n")
		b.WriteString(hintStyle.Render("a toggle auto-remove orphans · enter continue · esc back"))
	case screenOutput:
		b.WriteString(boxStyle.Render("Output paths"))
		b.WriteString("\n")
		b.WriteString(labelField("Output", m.outputInput, m.outputFocus == 0))
		b.WriteString("\n")
		b.WriteString(labelField("Report", m.reportInput, m.outputFocus == 1))
		b.WriteString("\n\n")
		fast := "off"
		if m.fast {
			fast = "on (gzip level 1)"
		}
		b.WriteString(fmt.Sprintf("Fast compress: %s  (press f to toggle)\n", fast))
		b.WriteString(hintStyle.Render("tab switch field · enter confirm · esc back"))
	case screenConfirm:
		b.WriteString(renderConfirm(m))
		if m.preview != nil {
			b.WriteString("\n")
			b.WriteString(renderTreeHeader(m.preview, treeTabBefore, false, m.treeGroupCursor))
			b.WriteString("\n")
			lines := backup.FormatTreeLines(m.preview, m.treeExpanded, m.width)
			maxLines := min(8, len(lines))
			for i := 0; i < maxLines; i++ {
				b.WriteString(lines[i])
				b.WriteString("\n")
			}
			if len(lines) > maxLines {
				b.WriteString(hintStyle.Render(fmt.Sprintf("  … +%d more lines in full tree after run", len(lines)-maxLines)))
				b.WriteString("\n")
			}
		}
	case screenRunning:
		pct := 0.0
		if m.progressTotal > 0 {
			pct = float64(m.progressCur) / float64(m.progressTotal)
		}
		b.WriteString(fmt.Sprintf("%s Sanitizing backup…\n\n", m.spinner.View()))
		b.WriteString(ascii.ProgressLoader(pct, m.width, time.Now()))
		b.WriteString("\n\n")
		b.WriteString(subtitleStyle.Render(fmt.Sprintf("%.0f%%  %d / %d objects", pct*100, m.progressCur, m.progressTotal)))
	case screenRestoreRunning:
		b.WriteString(fmt.Sprintf("%s\n\n", m.spinner.View()))
		b.WriteString(ascii.IndeterminateLoader(m.width, time.Now()))
		b.WriteString("\n\n")
		b.WriteString(boxStyle.Render(m.restoreStatus))
		b.WriteString("\n")
		b.WriteString(subtitleStyle.Render("kubectl cp → Restore CR → watch Ready"))
	case screenDone:
		title := "Sanitize complete"
		if m.flow == flowRestore || m.restoreStatus != "" && strings.Contains(m.restoreStatus, "Restore Ready") {
			title = "Restore complete"
		} else if m.flow == flowS3Pull && len(m.doneLines) > 0 {
			title = "S3 download complete"
		}
		b.WriteString(okStyle.Render(title))
		b.WriteString("\n\n")
		if m.flow == flowRestore || strings.Contains(m.restoreStatus, "Restore") {
			b.WriteString(m.restoreStatus)
			b.WriteString("\n\n")
			if m.flow == flowS3Pull {
				b.WriteString(hintStyle.Render("enter → sanitize this backup · esc menu"))
			} else {
				b.WriteString(hintStyle.Render("enter menu · q quit"))
			}
		} else if m.flow == flowS3Pull {
			for _, line := range m.doneLines {
				b.WriteString(line)
				b.WriteString("\n")
			}
			b.WriteString("\n")
			b.WriteString(hintStyle.Render("enter → sanitize this backup · esc menu"))
		} else {
			for _, line := range m.doneLines {
				b.WriteString(line)
				b.WriteString("\n")
			}
			b.WriteString("\n")
			if m.preview != nil {
				b.WriteString(renderTreeHeader(m.activePreview(), m.treeTab, m.previewAfter != nil, m.treeGroupCursor))
				b.WriteString("\n")
				b.WriteString(m.treeViewport.View())
				b.WriteString("\n")
				b.WriteString(hintStyle.Render("tab before/after · enter expand · j/k group · q quit"))
			} else {
				b.WriteString(hintStyle.Render("enter or q quit"))
			}
		}
	}

	b.WriteString("\n")
	return b.String()
}

func labelField(label string, ti textinput.Model, focused bool) string {
	style := blurredStyle
	if focused {
		style = focusedStyle
	}
	return fmt.Sprintf("%-8s %s", label+":", style.Render(ti.View()))
}

func renderInspectBrief(in *backup.InspectResult) string {
	if in == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(boxStyle.Render("Inspect summary"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s (%s) · %d objects · %d local refs stripped on sanitize\n",
		filepath.Base(in.Path), backup.HumanSize(in.InputSize), in.MemberCount, in.LocalArtifacts))
	downstream := 0
	for cid := range in.Clusters {
		if cid != "local" {
			downstream++
		}
	}
	b.WriteString(fmt.Sprintf("  %d downstream cluster(s) · %d fleet mappings", downstream, in.FleetMappings))
	if len(in.GhostIDs) > 0 {
		b.WriteString(warnStyle.Render(fmt.Sprintf(" · %d ghost ID(s)", len(in.GhostIDs))))
	}
	return b.String()
}

func renderInspect(in *backup.InspectResult) string {
	if in == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(boxStyle.Render("Inspect results"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  Backup     %s (%s)\n", in.Path, backup.HumanSize(in.InputSize)))
	b.WriteString(fmt.Sprintf("  Members    %d tar objects\n", in.MemberCount))
	b.WriteString(fmt.Sprintf("  Fleet      %d mappings, %d fleet-default JSON\n", in.FleetMappings, in.FleetDefault))
	b.WriteString(fmt.Sprintf("  Local refs %d (stripped on sanitize)\n", in.LocalArtifacts))

	b.WriteString("\n  Clusters:\n")
	for _, cid := range sortedIDs(in.Clusters) {
		if cid == "local" {
			continue
		}
		meta := in.Clusters[cid]
		b.WriteString(fmt.Sprintf("    %-10s %-22s %s\n", cid, meta.DisplayName, meta.Kind))
	}

	if len(in.GhostIDs) > 0 {
		b.WriteString("\n")
		b.WriteString(warnStyle.Render("  Ghost IDs (orphan path references):"))
		b.WriteString("\n")
		keys := sortedGhostKeys(in.GhostIDs)
		for _, k := range keys {
			b.WriteString(fmt.Sprintf("    %s  (%d paths)\n", k, in.GhostIDs[k]))
		}
	} else {
		b.WriteString("\n  ")
		b.WriteString(okStyle.Render("No ghost cluster IDs detected"))
		b.WriteString("\n")
	}
	return b.String()
}

func renderGhosts(in *backup.InspectResult, auto bool) string {
	var b strings.Builder
	b.WriteString(boxStyle.Render("Orphan / ghost cleanup"))
	b.WriteString("\n")
	state := "enabled"
	if !auto {
		state = "disabled"
	}
	b.WriteString(fmt.Sprintf("  Auto-remove orphan IDs: %s\n", state))
	if in != nil && len(in.GhostIDs) > 0 {
		b.WriteString("\n  Detected ghosts:\n")
		for _, k := range sortedGhostKeys(in.GhostIDs) {
			b.WriteString(fmt.Sprintf("    %s (%d paths)\n", k, in.GhostIDs[k]))
		}
	} else {
		b.WriteString("\n  No ghost IDs in this backup.\n")
	}
	return b.String()
}

func renderConfirm(m model) string {
	var b strings.Builder
	b.WriteString(boxStyle.Render("Confirm sanitize"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  Input:    %s\n", m.backupPath))
	b.WriteString(fmt.Sprintf("  Output:   %s\n", m.outputInput.Value()))
	if m.reportInput.Value() != "" {
		b.WriteString(fmt.Sprintf("  Report:   %s\n", m.reportInput.Value()))
	}
	if m.keepRKE1Only {
		b.WriteString("  Mode:     keep all RKE1 clusters\n")
	} else {
		b.WriteString(fmt.Sprintf("  Keep:     %s\n", strings.Join(m.selectedClusterIDs(), ", ")))
	}
	b.WriteString(fmt.Sprintf("  Orphans:  auto-remove=%v  fast=%v\n", m.autoOrphans, m.fast))
	b.WriteString("\n")
	b.WriteString(warnStyle.Render("  This writes a new tarball and does not modify the source."))
	b.WriteString("\n\n")
	b.WriteString(hintStyle.Render("enter run · n edit · esc back"))
	return b.String()
}

func buildDoneLines(res *backup.Result) []string {
	if res == nil {
		return nil
	}
	return []string{
		fmt.Sprintf("Output:  %s (%s)", res.OutputPath, backup.HumanSize(res.OutputSize)),
		fmt.Sprintf("Kept:    %d objects (%s)", len(res.Kept), backup.HumanSize(res.KeptBytes)),
		fmt.Sprintf("Removed: %d objects in %.1fs", len(res.Removed), res.Elapsed.Seconds()),
	}
}

func sortedIDs(clusters map[string]backup.ClusterMeta) []string {
	ids := make([]string, 0, len(clusters))
	for id := range clusters {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func sortedGhostKeys(ghosts map[string]int) []string {
	keys := make([]string, 0, len(ghosts))
	for k := range ghosts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func runInspect(path string) tea.Cmd {
	return func() tea.Msg {
		res, err := backup.InspectBackup(path)
		if err != nil {
			return inspectDoneMsg{err: err}
		}
		tree, err := backup.BuildInspectTree(path)
		if err != nil {
			return inspectDoneMsg{res: res, err: err}
		}
		return inspectDoneMsg{res: res, tree: tree}
	}
}

func waitSanitizeMsg(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
