package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// runeLen returns the number of runes (visible columns for ASCII/box-drawing) in s.
func runeLen(s string) int {
	return utf8.RuneCountInString(s)
}

// truncRunes truncates s to at most max runes (no mid-rune cut).
func truncRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

// padRunes pads s with spaces on the right until it has exactly n runes.
func padRunes(s string, n int) string {
	r := runeLen(s)
	if r >= n {
		return truncRunes(s, n)
	}
	return s + strings.Repeat(" ", n-r)
}

// visibleRuneCount returns rune count of s after stripping ANSI escape sequences.
func visibleRuneCount(s string) int {
	var out strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\033' && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) && s[i] != 'm' {
				i++
			}
			if i < len(s) {
				i++
			}
			continue
		}
		out.WriteByte(s[i])
		i++
	}
	return runeLen(out.String())
}

// centerIn pads s with spaces so it is centered in width runes (visible width used for s).
func centerIn(s string, width int) string {
	n := visibleRuneCount(s)
	if n >= width {
		return truncRunes(s, width)
	}
	left := (width - n) / 2
	right := width - n - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

const (
	defaultMpvSocket = "/tmp/mpvsocket"
)

func getMpvSocketPath() string {
	if p := os.Getenv("MPV_SOCKET"); p != "" {
		return p
	}
	return defaultMpvSocket
}

func getCDDevice() string {
	if p := os.Getenv("CD_DEVICE"); p != "" {
		return p
	}
	return "/dev/sr0"
}

// cdPresent returns true if a disc is present in the CD device.
// Uses sysfs size first (reliable when CD is already inserted), then blockdev. Does not use dd.
func cdPresent(ctx context.Context, device string) bool {
	// 1. Linux sysfs: /sys/block/<name>/size is non-zero when a disc is present; no exec, works when already inserted.
	base := filepath.Base(device)
	sizePath := filepath.Join("/sys/block", base, "size")
	if b, err := os.ReadFile(sizePath); err == nil {
		var size int64
		if _, err := fmt.Sscanf(strings.TrimSpace(string(b)), "%d", &size); err == nil {
			return size > 0
		}
	}

	// 2. blockdev --getsize64: reports "No medium found" when empty; works when disc is already in.
	ctx2, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx2, "blockdev", "--getsize64", device)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	var size int64
	if _, err := fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &size); err != nil {
		return false
	}
	return size > 0
}

var (
	mpvProc   *exec.Cmd
	mpvProcMu sync.Mutex
)

func startCDWatcher() {
	device := getCDDevice()
	socket := getMpvSocketPath()
	tick := 3 * time.Second
	var lastPresent bool
	for {
		present := cdPresent(context.Background(), device)
		mpvProcMu.Lock()
		if present && !lastPresent {
			// CD inserted: start mpv if not already running (TUI attaches to this instance via same socket)
			if mpvProc == nil {
				mpvProc = exec.Command("mpv", "cdda://",
					"--cdda-device="+device,
					"--input-ipc-server="+socket,
					"--no-terminal")
				mpvProc.Stdout = nil
				mpvProc.Stderr = nil
				if err := mpvProc.Start(); err != nil {
					mpvProc = nil
				} else {
					cmd := mpvProc
					go func() {
						cmd.Wait()
						mpvProcMu.Lock()
						if mpvProc == cmd {
							mpvProc = nil
						}
						mpvProcMu.Unlock()
					}()
				}
			}
		}
		if !present && lastPresent {
			// CD removed: kill mpv
			if mpvProc != nil && mpvProc.Process != nil {
				_ = mpvProc.Process.Kill()
				mpvProc = nil
			}
		}
		lastPresent = present
		mpvProcMu.Unlock()
		time.Sleep(tick)
	}
}

// killMpvIfSpawned kills the mpv process started by the program, if any. Call on TUI exit.
func killMpvIfSpawned() {
	mpvProcMu.Lock()
	defer mpvProcMu.Unlock()
	if mpvProc != nil && mpvProc.Process != nil {
		_ = mpvProc.Process.Kill()
		mpvProc = nil
	}
}

// palette holds colours for one theme.
type palette struct {
	PanelBorder   lipgloss.Color
	TitleAccent   lipgloss.Color
	TextPrimary   lipgloss.Color
	FlareColor    lipgloss.Color
	StatusPlay    lipgloss.Color
	StatusPause   lipgloss.Color
	StatusDisc    lipgloss.Color
	ProgressFull  lipgloss.Color
	ProgressEmpty lipgloss.Color
}

// palettes: 0 = pastel yellow, 1 = blue, 2 = green, 3 = rose (brighter text throughout)
var palettes = []palette{
	{
		PanelBorder:   lipgloss.Color("#FBC02D"),
		TitleAccent:   lipgloss.Color("#F57F17"),
		TextPrimary:   lipgloss.Color("#8D6E63"),
		FlareColor:    lipgloss.Color("#FFB300"),
		StatusPlay:    lipgloss.Color("#558B2F"),
		StatusPause:   lipgloss.Color("#F9A825"),
		StatusDisc:    lipgloss.Color("#A1887F"),
		ProgressFull:  lipgloss.Color("#FBC02D"),
		ProgressEmpty: lipgloss.Color("#FFF59D"),
	},
	{
		PanelBorder:   lipgloss.Color("#42A5F5"),
		TitleAccent:   lipgloss.Color("#1E88E5"),
		TextPrimary:   lipgloss.Color("#90A4AE"),
		FlareColor:    lipgloss.Color("#64B5F6"),
		StatusPlay:    lipgloss.Color("#66BB6A"),
		StatusPause:   lipgloss.Color("#FFA726"),
		StatusDisc:    lipgloss.Color("#B0BEC5"),
		ProgressFull:  lipgloss.Color("#42A5F5"),
		ProgressEmpty: lipgloss.Color("#B3E5FC"),
	},
	{
		PanelBorder:   lipgloss.Color("#66BB6A"),
		TitleAccent:   lipgloss.Color("#43A047"),
		TextPrimary:   lipgloss.Color("#A5D6A7"),
		FlareColor:    lipgloss.Color("#81C784"),
		StatusPlay:    lipgloss.Color("#9CCC65"),
		StatusPause:   lipgloss.Color("#FFB74D"),
		StatusDisc:    lipgloss.Color("#C5E1A5"),
		ProgressFull:  lipgloss.Color("#66BB6A"),
		ProgressEmpty: lipgloss.Color("#C8E6C9"),
	},
	{
		PanelBorder:   lipgloss.Color("#EC407A"),
		TitleAccent:   lipgloss.Color("#D81B60"),
		TextPrimary:   lipgloss.Color("#F48FB1"),
		FlareColor:    lipgloss.Color("#F06292"),
		StatusPlay:    lipgloss.Color("#66BB6A"),
		StatusPause:   lipgloss.Color("#FFB74D"),
		StatusDisc:    lipgloss.Color("#F8BBD9"),
		ProgressFull:  lipgloss.Color("#EC407A"),
		ProgressEmpty: lipgloss.Color("#FCE4EC"),
	},
}

type errMsg struct {
	err error
}

func (e errMsg) Error() string {
	if e.err == nil {
		return ""
	}
	return e.err.Error()
}

type model struct {
	status          string
	err             string
	chapter         int
	chapters        int
	positionSeconds float64
	durationSeconds float64
	trackTitle      string
	width           int
	height          int
	repeat          bool
	speed           float64
	cavaBars        []int // from cava file if present
	tickCount       int   // for placeholder visualizer animation
	paletteIndex    int   // 0-3, cycle with [c]
}

type statusMsg struct {
	playing         bool
	chapter         int
	chapters        int
	positionSeconds float64
	durationSeconds float64
	trackTitle      string
	speed           float64
	err             error
}

type tickMsg struct{}
type cavaMsg []int

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

func initModel() model {
	return model{
		status: "Disconnected",
		repeat: true,
	}
}

func sendMPVCommand(cmd []interface{}) error {
	conn, err := net.Dial("unix", getMpvSocketPath())
	if err != nil {
		return err
	}
	defer conn.Close()

	request := map[string]interface{}{
		"command": cmd,
	}
	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(request); err != nil {
		return err
	}

	// Always read and discard a single response so mpv
	// doesn't get a broken pipe when writing the reply.
	var resp map[string]interface{}
	decoder := json.NewDecoder(conn)
	if err := decoder.Decode(&resp); err != nil {
		return err
	}

	if errStr, ok := resp["error"].(string); ok && errStr != "success" {
		return fmt.Errorf("mpv: %s", errStr)
	}

	return nil
}

func sendCommandCmd(cmd []interface{}) tea.Cmd {
	return func() tea.Msg {
		if err := sendMPVCommand(cmd); err != nil {
			return errMsg{err: err}
		}
		return nil
	}
}

func queryStatusCmd() tea.Cmd {
	return func() tea.Msg {
		return queryStatus()
	}
}

// cavaBarFile is where we write the latest cava frame (space-separated 0-8 bar heights) for the TUI to read.
const cavaBarFile = "/tmp/cdplayer_cava.txt"

// cavaConfigContent is a minimal cava config for raw ASCII output to stdout (bars; bar_delimiter=59 ';', frame_delimiter=10 '\n').
// Pulse is used by default (works with PipeWire via pipewire-pulse, or native PulseAudio).
const cavaConfigContent = `[general]
framerate = 25
bars = 48
[input]
method = pulse
source = auto
[output]
method = raw
raw_target = /dev/stdout
data_format = ascii
bar_delimiter = 59
frame_delimiter = 10
ascii_max_range = 1000
`

func startCavaCmd() tea.Cmd {
	return func() tea.Msg {
		dir := os.TempDir()
		configPath := filepath.Join(dir, "cdplayer_cava_config")
		if err := os.WriteFile(configPath, []byte(cavaConfigContent), 0600); err != nil {
			return nil
		}
		cmd := exec.Command("cava", "-p", configPath)
		cmd.Stderr = nil
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return nil
		}
		if err := cmd.Start(); err != nil {
			return nil
		}
		go func() {
			defer cmd.Wait()
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				line := scanner.Text()
				parts := strings.Split(line, ";")
				var bars []string
				for _, p := range parts {
					p = strings.TrimSpace(p)
					if p == "" {
						continue
					}
					v, err := strconv.Atoi(p)
					if err != nil {
						continue
					}
					// ascii_max_range 1000 -> scale to 0-8
					h := v * 8 / 1000
					if h > 8 {
						h = 8
					}
					bars = append(bars, strconv.Itoa(h))
				}
				if len(bars) > 0 {
					_ = os.WriteFile(cavaBarFile, []byte(strings.Join(bars, " ")+"\n"), 0644)
				}
			}
		}()
		return nil
	}
}

func readCavaCmd() tea.Cmd {
	return func() tea.Msg {
		data, err := os.ReadFile(cavaBarFile)
		if err != nil {
			return cavaMsg(nil)
		}
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		if len(lines) == 0 {
			return cavaMsg(nil)
		}
		lastLine := lines[len(lines)-1]
		fields := strings.Fields(lastLine)
		bars := make([]int, 0, len(fields))
		for _, f := range fields {
			n, err := strconv.Atoi(f)
			if err != nil || n < 0 || n > 8 {
				continue
			}
			bars = append(bars, n)
		}
		return cavaMsg(bars)
	}
}

func queryStatus() statusMsg {
	conn, err := net.Dial("unix", getMpvSocketPath())
	if err != nil {
		return statusMsg{err: err}
	}
	defer conn.Close()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	get := func(prop string) (interface{}, error) {
		request := map[string]interface{}{
			"command": []interface{}{"get_property", prop},
		}
		if err := encoder.Encode(request); err != nil {
			return nil, err
		}

		var resp map[string]interface{}
		if err := decoder.Decode(&resp); err != nil {
			return nil, err
		}

		if errStr, ok := resp["error"].(string); ok && errStr != "success" {
			return nil, fmt.Errorf("mpv: %s", errStr)
		}

		return resp["data"], nil
	}

	var s statusMsg

	var (
		globalTimePos  float64
		globalDuration float64
		haveTimePos    bool
		haveDuration   bool
	)

	if v, err := get("pause"); err == nil {
		if b, ok := v.(bool); ok {
			s.playing = !b
		}
	}

	if v, err := get("chapter"); err == nil {
		if f, ok := v.(float64); ok {
			s.chapter = int(f)
		}
	}

	if v, err := get("chapters"); err == nil {
		if f, ok := v.(float64); ok {
			s.chapters = int(f)
		}
	}

	if v, err := get("time-pos"); err == nil {
		if f, ok := v.(float64); ok {
			globalTimePos = f
			haveTimePos = true
		}
	}

	if v, err := get("duration"); err == nil {
		if f, ok := v.(float64); ok {
			globalDuration = f
			haveDuration = true
		}
	}

	if v, err := get("speed"); err == nil {
		if f, ok := v.(float64); ok {
			s.speed = f
		}
	}

	// Derive per-chapter timing from chapter metadata so each track has its own duration.
	if v, err := get("chapter-list"); err == nil {
		if list, ok := v.([]interface{}); ok && s.chapters > 0 && s.chapter >= 0 && s.chapter < len(list) && haveTimePos {
			startTimes := make([]float64, len(list))
			for i, item := range list {
				if m, ok := item.(map[string]interface{}); ok {
					if t, ok := m["time"].(float64); ok {
						startTimes[i] = t
					}
					// Pick up a title for the current chapter if present in CD metadata.
					if i == s.chapter {
						if title, ok := m["title"].(string); ok && title != "" {
							s.trackTitle = title
						} else if name, ok := m["name"].(string); ok && name != "" {
							s.trackTitle = name
						}
					}
				}
			}

			chIdx := s.chapter
			chStart := startTimes[chIdx]

			// End is next chapter start or overall duration as fallback.
			chEnd := chStart
			if chIdx+1 < len(startTimes) {
				chEnd = startTimes[chIdx+1]
			} else if haveDuration {
				chEnd = globalDuration
			}

			chDur := chEnd - chStart
			if chDur < 0 {
				chDur = 0
			}

			posInChapter := globalTimePos - chStart
			if posInChapter < 0 {
				posInChapter = 0
			}
			if chDur > 0 && posInChapter > chDur {
				posInChapter = chDur
			}

			s.positionSeconds = posInChapter
			s.durationSeconds = chDur

			return s
		}
	}

	// Fallback: no chapter metadata; use global timing.
	if haveTimePos {
		s.positionSeconds = globalTimePos
	}
	if haveDuration {
		s.durationSeconds = globalDuration
	}

	return s
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		tea.ClearScreen,
		startCavaCmd(),
		tickCmd(),
		queryStatusCmd(),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case " ":
			return m, sendCommandCmd([]interface{}{"cycle", "pause"})

		case "n", "right":
			return m, sendCommandCmd([]interface{}{"add", "chapter", 1})

		case "p", "left":
			return m, sendCommandCmd([]interface{}{"add", "chapter", -1})

		case "up":
			return m, sendCommandCmd([]interface{}{"multiply", "speed", 1.1})

		case "down":
			return m, sendCommandCmd([]interface{}{"multiply", "speed", 0.86})

		case "r":
			m.repeat = !m.repeat
			val := "no"
			if m.repeat {
				val = "inf"
			}
			return m, sendCommandCmd([]interface{}{"set_property", "loop-playlist", val})

		case "c":
			m.paletteIndex = (m.paletteIndex + 1) % len(palettes)
			return m, nil
		}

	case errMsg:
		if msg.err != nil {
			m.err = msg.Error()
		}
		return m, nil

	case tickMsg:
		m.tickCount++
		val := "no"
		if m.repeat {
			val = "inf"
		}
		return m, tea.Batch(
			queryStatusCmd(),
			sendCommandCmd([]interface{}{"set_property", "loop-playlist", val}),
			readCavaCmd(),
			tickCmd(),
		)

	case cavaMsg:
		m.cavaBars = msg
		return m, nil

	case statusMsg:
		if msg.err != nil {
			m.status = "Disconnected"
			m.err = msg.err.Error()
			return m, nil
		}

		m.err = ""

		if msg.playing {
			m.status = "Playing"
		} else {
			m.status = "Paused"
		}

		m.chapter = msg.chapter
		m.chapters = msg.chapters
		m.positionSeconds = msg.positionSeconds
		m.durationSeconds = msg.durationSeconds
		m.trackTitle = msg.trackTitle
		m.speed = msg.speed
		return m, nil
	}

	return m, nil
}

func (m model) View() string {
	var b strings.Builder

	// If we don't yet know the terminal size, show a minimal view.
	if m.width == 0 || m.height == 0 {
		b.WriteString("CD Player\n")
		b.WriteString("Status: " + m.status + "\n")
		return b.String()
	}

	// Compute a 16:9 content area centered in the terminal.
	contentWidth := m.width
	contentHeight := int(float64(contentWidth) * 9.0 / 16.0)
	if contentHeight > m.height {
		contentHeight = m.height
		contentWidth = int(float64(contentHeight) * 16.0 / 9.0)
	}
	if contentWidth > m.width {
		contentWidth = m.width
	}
	if contentWidth < 82 {
		contentWidth = 82
	}
	if contentWidth > m.width {
		contentWidth = m.width
	}
	if contentHeight < 12 {
		contentHeight = 12
	}

	innerWidth := contentWidth - 2
	hPad := (m.width - contentWidth) / 2
	if hPad < 0 {
		hPad = 0
	}

	p := palettes[m.paletteIndex]
	curPanelStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.PanelBorder).
		Foreground(p.TextPrimary)
	curTitleStyle := lipgloss.NewStyle().Bold(true).Foreground(p.TitleAccent)
	curFlareStyle := lipgloss.NewStyle().Foreground(p.FlareColor)

	line := func(inner string) {
		centered := centerIn(inner, innerWidth)
		b.WriteString(centered)
		b.WriteString("\n")
	}

	// Line 1: [q] Quit right-aligned so it doesn't interfere with title
	quitLabel := " [q] Quit "
	quitPad := innerWidth - runeLen(quitLabel)
	if quitPad < 0 {
		quitPad = 0
	}
	b.WriteString(strings.Repeat(" ", quitPad) + quitLabel + "\n")

	// Line 2: CD Player centered (with flares)
	leftFlare := "  ✦ ✦  "
	centerTitle := " CD PLAYER "
	rightFlare := "  ✦ ✦  "
	titleContent := curFlareStyle.Render(leftFlare) + curTitleStyle.Render(centerTitle) + curFlareStyle.Render(rightFlare)
	titleContentRunes := runeLen(leftFlare) + runeLen(centerTitle) + runeLen(rightFlare)
	titleLeftPad := (innerWidth - titleContentRunes) / 2
	if titleLeftPad < 0 {
		titleLeftPad = 0
	}
	titleRightPad := innerWidth - titleContentRunes - titleLeftPad
	if titleRightPad < 0 {
		titleRightPad = 0
	}
	b.WriteString(strings.Repeat(" ", titleLeftPad) + titleContent + strings.Repeat(" ", titleRightPad) + "\n")

	// Status line (centered)
	statusWord := m.status
	statusFg := p.StatusDisc
	switch m.status {
	case "Playing":
		statusFg = p.StatusPlay
	case "Paused":
		statusFg = p.StatusPause
	}
	statusWordStyle := lipgloss.NewStyle().Bold(true).Foreground(statusFg)
	statusContent := "STATUS: " + statusWordStyle.Render(statusWord)
	line(statusContent)

	// Track info (centered)
	trackInfo := "TRACK: -- / --"
	if m.chapters > 0 {
		trackInfo = fmt.Sprintf("TRACK: %02d / %02d", m.chapter+1, m.chapters)
	}
	line(trackInfo)

	// Title / track name (centered)
	if m.trackTitle != "" {
		maxTitleRunes := innerWidth
		if runeLen(m.trackTitle) > maxTitleRunes {
			if maxTitleRunes >= 3 {
				trackTitle := truncRunes(m.trackTitle, maxTitleRunes-3) + "..."
				line(trackTitle)
			} else {
				line(truncRunes(m.trackTitle, maxTitleRunes))
			}
		} else {
			line(m.trackTitle)
		}
	} else {
		line("")
	}

	// Time and speed (centered, one line)
	if m.durationSeconds > 0 {
		timeStr := fmt.Sprintf("TIME: %s / %s", formatTime(m.positionSeconds), formatTime(m.durationSeconds))
		speedStr := fmt.Sprintf("Speed: %.2fx", m.speed)
		if m.speed <= 0 {
			speedStr = "Speed: 1.00x"
		}
		line(timeStr + "    " + speedStr)

		barWidth := int(float64(innerWidth) * 0.8)
		if barWidth < 10 {
			barWidth = 10
		}
		bar := progressBarSimple(m.positionSeconds, m.durationSeconds, barWidth, p.ProgressFull, p.ProgressEmpty, p.TextPrimary)
		b.WriteString(centerIn(bar, innerWidth) + "\n")
	} else {
		speedStr := fmt.Sprintf("Speed: %.2fx", m.speed)
		if m.speed <= 0 {
			speedStr = "Speed: 1.00x"
		}
		line(speedStr)
		line("")
	}

	line("")

	// Visualizer: single wavy-line decoration (cava or placeholder)
	visLine := renderVisualizer(m.cavaBars, m.tickCount, innerWidth, p.ProgressFull)
	b.WriteString(visLine + "\n")

	line("")
	// Controls line 1: Prev, Play, Next (with letter keys)
	controls1 := "← [p] Prev    [SPACE] Play    → [n] Next"
	line(controls1)
	// Controls line 2: Speed+, Speed-, Repeat
	controls2 := "↑ Speed+    ↓ Speed-    [r] Repeat    [c] Palette"
	line(controls2)

	if m.err != "" {
		line("ERROR: " + m.err)
	}

	innerContent := b.String()

	// Wrap in panel and center (use current palette style)
	rendered := curPanelStyle.Width(innerWidth).Render(innerContent)
	marginStyle := lipgloss.NewStyle().MarginLeft(hPad)
	return marginStyle.Render(rendered)
}
func formatTime(sec float64) string {
	total := int(sec + 0.5)
	if total < 0 {
		total = 0
	}

	minutes := total / 60
	seconds := total % 60

	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

func progressBar(pos, dur float64, width int) string {
	if dur <= 0 || width <= 0 {
		return ""
	}

	ratio := pos / dur
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}

	filled := int(ratio * float64(width))
	if filled > width {
		filled = width
	}

	return "[" + strings.Repeat("=", filled) + strings.Repeat(" ", width-filled) + "]"
}

// progressBarSimple draws a minimal bar with a small circular dot for position: ━━━●━━━━━━ (80% view width)
func progressBarSimple(pos, dur float64, width int, progressFull, progressEmpty, textPrimary lipgloss.Color) string {
	if dur <= 0 || width <= 0 {
		return ""
	}
	ratio := pos / dur
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	// One rune for the dot; rest are ━ segments
	barArea := width - 1
	if barArea < 0 {
		barArea = 0
	}
	leftCount := int(ratio * float64(barArea))
	if leftCount > barArea {
		leftCount = barArea
	}
	rightCount := barArea - leftCount

	leftStyle := lipgloss.NewStyle().Foreground(progressFull)
	dotStyle := lipgloss.NewStyle().Foreground(progressFull)
	rightStyle := lipgloss.NewStyle().Foreground(progressEmpty)
	dot := "●"
	s := leftStyle.Render(strings.Repeat("━", leftCount)) + dotStyle.Render(dot) + rightStyle.Render(strings.Repeat("━", rightCount))
	return s
}

// visBars are vertical block chars for one wavy line (U+2581–U+2588).
var visBars = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// renderVisualizer returns a single wavy-line decoration. If cavaBars has data, each column uses that bar height (0–8); else a placeholder wave.
func renderVisualizer(cavaBars []int, tickCount int, width int, accent lipgloss.Color) string {
	style := lipgloss.NewStyle().Foreground(accent)
	n := width
	if n > 60 {
		n = 60
	}
	if n < 10 {
		n = 10
	}
	var runes []rune
	if len(cavaBars) >= n {
		for i := 0; i < n; i++ {
			h := 0
			if i < len(cavaBars) {
				h = cavaBars[i]
			}
			if h > 8 {
				h = 8
			}
			idx := h
			if idx >= len(visBars) {
				idx = len(visBars) - 1
			}
			runes = append(runes, visBars[idx])
		}
	} else {
		for i := 0; i < n; i++ {
			x := float64(i) / float64(n) * 4 * math.Pi
			t := float64(tickCount) * 0.3
			h := 0.5 + 0.5*math.Sin(x+t)
			idx := int(h * 7)
			if idx < 0 {
				idx = 0
			}
			if idx >= len(visBars) {
				idx = len(visBars) - 1
			}
			runes = append(runes, visBars[idx])
		}
	}
	return centerIn(style.Render(string(runes)), width)
}

func main() {
	go startCDWatcher()
	p := tea.NewProgram(initModel())
	if _, err := p.Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
	killMpvIfSpawned()
}

