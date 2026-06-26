package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aeltai/rancher-migrate/internal/ascii"
	"github.com/aeltai/rancher-migrate/internal/backup"
	"github.com/aeltai/rancher-migrate/internal/config"
	"github.com/aeltai/rancher-migrate/internal/k8s"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
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
	screenGhosts
	screenOutput
	screenConfirm
	screenRunning
	screenDone
	screenRestoreInput
	screenRestoreRunning
)

type flow int

const (
	flowSanitize flow = iota
	flowInspectOnly
	flowRestore
)

type menuItem struct {
	title, desc string
}

func (i menuItem) Title() string       { return i.title }
func (i menuItem) Description() string { return i.desc }
func (i menuItem) FilterValue() string { return i.title }

type clusterItem struct {
	id, display, kind string
}

func (i clusterItem) Title() string       { return i.id + "  " + i.display }
func (i clusterItem) Description() string { return "kind=" + i.kind }
func (i clusterItem) FilterValue() string { return i.id + " " + i.display }

type inspectDoneMsg struct {
	res *backup.InspectResult
	err error
}

type sanitizeDoneMsg struct {
	res *backup.Result
	err error
}

type progressMsg struct {
	current, total int
}

type splashTickMsg time.Time

type restoreDoneMsg struct {
	err        error
	backupName string
}

type model struct {
	width, height int
	screen        screen
	flow          flow
	err           string
	cfg           config.Config
	splashAt      time.Time
	animations    bool

	menu         list.Model
	modeList     list.Model
	clusterList  list.Model
	input        textinput.Model
	outputInput  textinput.Model
	reportInput  textinput.Model
	outputFocus  int // 0=output, 1=report

	spinner   spinner.Model
	inspect   *backup.InspectResult
	result    *backup.Result
	backupPath string

	keepRKE1Only bool
	fast         bool
	autoOrphans  bool

	progressCur   int
	progressTotal int
	doneLines     []string
	sanitizeCh    chan tea.Msg
	restoreLocal  string
	restoreStatus string
}

func newModel(cfg config.Config) model {
	menuItems := []list.Item{
		menuItem{"Sanitize backup", "Guided wizard — inspect, pick cluster, write tarball"},
		menuItem{"Inspect only", "Read-only analysis of clusters and ghost IDs"},
		menuItem{"Restore to cluster", "kubectl cp + Restore CR (uses config kubeconfig)"},
		menuItem{"Quit", "Exit rancher-migrate"},
	}
	menu := list.New(menuItems, list.NewDefaultDelegate(), 0, 0)
	menu.Title = "What would you like to do?"
	menu.SetShowStatusBar(false)
	menu.SetFilteringEnabled(false)
	menu.SetShowHelp(true)

	modeItems := []list.Item{
		menuItem{"Keep one cluster", "Remove all other downstream clusters (typical migration)"},
		menuItem{"Keep all RKE1", "Remove imported / RKE2 clusters only"},
	}
	modeList := list.New(modeItems, list.NewDefaultDelegate(), 0, 0)
	modeList.Title = "Cluster retention mode"
	modeList.SetShowStatusBar(false)
	modeList.SetFilteringEnabled(false)

	clusterList := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	clusterList.Title = "Select cluster to keep"
	clusterList.SetShowStatusBar(false)
	clusterList.SetFilteringEnabled(true)

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
		screen:      startScreen,
		splashAt:    time.Now(),
		animations:  cfg.UI.Animations,
		cfg:         cfg,
		menu:        menu,
		modeList:    modeList,
		clusterList: clusterList,
		input:       ti,
		outputInput: out,
		reportInput: rep,
		spinner:     sp,
		autoOrphans: cfg.AutoOrphansEnabled(),
		fast:        cfg.Defaults.Fast,
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
		m.menu.SetSize(msg.Width-4, min(12, msg.Height-8))
		m.modeList.SetSize(msg.Width-4, min(8, msg.Height-8))
		m.clusterList.SetSize(msg.Width-4, min(14, msg.Height-10))
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

	case restoreDoneMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
			m.screen = screenRestoreInput
			return m, nil
		}
		m.restoreStatus = fmt.Sprintf("Restore CR applied for %s. Run: rancher-migrate restore status", msg.backupName)
		m.screen = screenDone
		return m, nil

	case inspectDoneMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
			m.screen = screenInput
			return m, nil
		}
		m.inspect = msg.res
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
		case screenInspectView:
			return m.updateInspectView(msg)
		case screenMode:
			return m.updateMode(msg)
		case screenCluster:
			return m.updateCluster(msg)
		case screenGhosts:
			return m.updateGhosts(msg)
		case screenOutput:
			return m.updateOutput(msg)
		case screenConfirm:
			return m.updateConfirm(msg)
		case screenDone:
			if msg.String() == "enter" || msg.String() == "q" {
				if m.flow == flowRestore {
					m.screen = screenMenu
					m.flow = flowSanitize
					m.err = ""
					return m, nil
				}
				return m, tea.Quit
			}
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
		m.flow = flowSanitize
		m.screen = screenInput
		m.input.SetValue(m.backupPath)
		m.input.Focus()
	case 1:
		m.flow = flowInspectOnly
		m.screen = screenInput
		m.input.SetValue(m.backupPath)
		m.input.Focus()
	case 2:
		m.flow = flowRestore
		m.screen = screenRestoreInput
		m.input.SetValue(m.restoreLocal)
		m.input.Placeholder = "/path/to/sanitized-backup.tar.gz"
		m.input.Focus()
	case 3:
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
			m.err = "restore.kubeconfig not set — run: rancher-migrate config init"
			return m, nil
		}
		m.restoreLocal = path
		m.screen = screenRestoreRunning
		m.restoreStatus = "Copying backup to operator pod…"
		ch := make(chan tea.Msg, 4)
		go func() {
			client := k8s.NewClient(m.cfg.Restore)
			ctx := context.Background()
			name, err := client.CopyBackup(ctx, path, m.cfg.Restore.OperatorNamespace,
				m.cfg.Restore.BackupPodLabel, m.cfg.Restore.BackupContainerPath)
			if err != nil {
				ch <- restoreDoneMsg{err: err}
				return
			}
			if err := client.ApplyRestore(ctx, m.cfg.Restore, name); err != nil {
				ch <- restoreDoneMsg{err: err}
				return
			}
			ch <- restoreDoneMsg{backupName: name}
		}()
		return m, waitSanitizeMsg(ch)
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
	case "enter":
		if m.flow == flowInspectOnly {
			return m, tea.Quit
		}
		m.screen = screenMode
	}
	return m, nil
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
		m.screen = screenGhosts
	} else {
		m.clusterList = populateClusterList(m.inspect, m.width, m.height)
		m.screen = screenCluster
	}
	return m, nil
}

func (m model) updateCluster(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.screen = screenMode
		return m, nil
	}
	if msg.String() != "enter" {
		var cmd tea.Cmd
		m.clusterList, cmd = m.clusterList.Update(msg)
		return m, cmd
	}
	if m.clusterList.SelectedItem() == nil {
		return m, nil
	}
	m.screen = screenGhosts
	return m, nil
}

func (m model) updateGhosts(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if m.keepRKE1Only {
			m.screen = screenMode
		} else {
			m.screen = screenCluster
		}
	case "a":
		m.autoOrphans = !m.autoOrphans
	case "enter":
		m.applyOutputDefaults()
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

func (m model) selectedClusterID() string {
	if item, ok := m.clusterList.SelectedItem().(clusterItem); ok {
		return item.id
	}
	return ""
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
		opts.KeepCluster = m.selectedClusterID()
	}
	return opts
}

func (m model) View() string {
	if m.screen == screenSplash {
		return lipgloss.NewStyle().
			Width(m.width).
			Render(ascii.SplashFrame(time.Now()) + "\n" + hintStyle.Render("press enter to continue"))
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
			kc = "(not set — run: rancher-migrate config init)"
		}
		b.WriteString(fmt.Sprintf("Kubeconfig: %s\n", kc))
		b.WriteString(hintStyle.Render("enter start restore · esc menu"))
	case screenLoading:
		b.WriteString(fmt.Sprintf("%s Inspecting backup…\n", m.spinner.View()))
		b.WriteString(subtitleStyle.Render(m.backupPath))
	case screenInspectView:
		b.WriteString(renderInspect(m.inspect))
		if m.flow == flowSanitize {
			b.WriteString("\n")
			b.WriteString(hintStyle.Render("enter continue to sanitize · esc menu"))
		} else {
			b.WriteString("\n")
			b.WriteString(hintStyle.Render("enter quit · esc menu"))
		}
	case screenMode:
		b.WriteString(m.modeList.View())
	case screenCluster:
		b.WriteString(m.clusterList.View())
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
	case screenRunning:
		b.WriteString(fmt.Sprintf("%s %s Sanitizing backup…\n\n", ascii.ProgressCow(time.Now()), m.spinner.View()))
		b.WriteString(renderStaticBar(m.progressCur, m.progressTotal))
		b.WriteString("\n")
		pct := 0.0
		if m.progressTotal > 0 {
			pct = float64(m.progressCur) / float64(m.progressTotal) * 100
		}
		b.WriteString(subtitleStyle.Render(fmt.Sprintf("%.0f%%  %d / %d objects", pct, m.progressCur, m.progressTotal)))
	case screenRestoreRunning:
		b.WriteString(fmt.Sprintf("%s %s\n\n", ascii.ProgressCow(time.Now()), m.spinner.View()))
		b.WriteString(boxStyle.Render(m.restoreStatus))
		b.WriteString("\n")
		b.WriteString(subtitleStyle.Render("kubectl cp → Restore CR apply"))
	case screenDone:
		title := "Sanitize complete"
		if m.flow == flowRestore {
			title = "Restore started"
		}
		b.WriteString(okStyle.Render(title))
		b.WriteString("\n\n")
		if m.flow == flowRestore {
			b.WriteString(m.restoreStatus)
			b.WriteString("\n\n")
			b.WriteString(hintStyle.Render("enter menu · q quit"))
		} else {
			for _, line := range m.doneLines {
				b.WriteString(line)
				b.WriteString("\n")
			}
			b.WriteString("\n")
			b.WriteString(hintStyle.Render("enter or q quit"))
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
		b.WriteString(fmt.Sprintf("  Keep:     %s\n", m.selectedClusterID()))
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

func populateClusterList(in *backup.InspectResult, width, height int) list.Model {
	items := make([]list.Item, 0)
	for _, cid := range sortedIDs(in.Clusters) {
		if cid == "local" {
			continue
		}
		meta := in.Clusters[cid]
		items = append(items, clusterItem{id: cid, display: meta.DisplayName, kind: meta.Kind})
	}
	w := 60
	h := 14
	if width > 4 {
		w = width - 4
	}
	if height > 10 {
		h = min(14, height-10)
	}
	l := list.New(items, list.NewDefaultDelegate(), w, h)
	l.Title = "Select cluster to keep"
	l.SetShowStatusBar(false)
	return l
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
		return inspectDoneMsg{res: res, err: err}
	}
}

func renderStaticBar(current, total int) string {
	total = max(1, total)
	ratio := float64(current) / float64(total)
	width := 36
	filled := int(ratio * float64(width))
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return bar
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
