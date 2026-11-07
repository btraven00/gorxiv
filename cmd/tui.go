package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/btraven00/gorxiv/epmc"
)

type item struct{ r *epmc.Record }

func (i item) Title() string       { return epmc.StripTags(i.r.Title) }
func (i item) Description() string { return meta(i.r) }
func (i item) FilterValue() string { return i.Title() + " " + i.r.AuthorString + " " + shortID(i.r) }

type (
	statusMsg  string
	resultsMsg struct {
		recs       []epmc.Record
		hits       int
		raw, query string // raw: what the user typed; query: composed with flag filters
		keepPos    bool
	}
)

type tuiModel struct {
	c          *epmc.Client
	list       list.Model
	vp         viewport.Model
	in         textinput.Model
	detail     bool
	searching  bool
	raw, query string
	sort       string
	hits       int
	status     string
}

var (
	keyEnter  = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter/click", "details"))
	keySearch = key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "new search"))
	keyMore   = key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "load more"))
	keyPDF    = key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "pdf"))
	keyOpen   = key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open pdf"))
	keyWeb    = key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "browser"))
	keyBack   = key.NewBinding(key.WithKeys("esc", "q", "backspace", "left", "h"), key.WithHelp("esc", "back"))
)

func runTUI(c *epmc.Client, recs []epmc.Record, hits int, raw, query, sort string) error {
	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	l.AdditionalShortHelpKeys = func() []key.Binding { return []key.Binding{keyEnter, keySearch, keyMore, keyPDF, keyOpen, keyWeb} }
	in := textinput.New()
	in.Prompt = "search: "
	m := tuiModel{c: c, list: l, in: in, sort: sort}
	m.setResults(resultsMsg{recs: recs, hits: hits, raw: raw, query: query})
	if raw == "" { // no query yet: start in the search box
		m.searching = true
		m.in.Focus()
	}
	_, err := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
	return err
}

func (m tuiModel) Init() tea.Cmd { return nil }

func (m tuiModel) selected() *epmc.Record {
	if it, ok := m.list.SelectedItem().(item); ok {
		return it.r
	}
	return nil
}

func (m *tuiModel) setResults(r resultsMsg) {
	pos := m.list.Index()
	items := make([]list.Item, len(r.recs))
	for i := range r.recs {
		items[i] = item{&r.recs[i]}
	}
	m.list.SetItems(items)
	m.list.ResetFilter()
	if r.keepPos {
		m.list.Select(pos)
	} else {
		m.list.Select(0)
	}
	m.raw, m.query, m.hits = r.raw, r.query, r.hits
	m.list.Title = "gorxiv"
	if r.raw != "" {
		m.list.Title = fmt.Sprintf("%s — %d of %d", r.raw, len(r.recs), r.hits)
	}
}

// search re-runs the query for limit records. ponytail: "load more" refetches
// from the first page (Search has no resume); fine for hundreds, expose the cursor if it drags.
func (m tuiModel) search(raw string, limit int, keepPos bool) tea.Cmd {
	return func() tea.Msg {
		q, err := epmc.Compose(raw, sf) // flag filters (--oa, --from, …) stick across searches
		if err != nil {
			return statusMsg(err.Error())
		}
		recs, hits, err := m.c.Search(context.Background(), q, m.sort, limit, true)
		if err != nil {
			return statusMsg("search: " + err.Error())
		}
		return resultsMsg{recs, hits, raw, q, keepPos}
	}
}

// itemAt maps a mouse row to a list index, or -1. Relies on the default
// delegate's fixed item height and the list's title/status bars above the items.
func (m tuiModel) itemAt(y int) int {
	top := lipgloss.Height(m.list.Styles.TitleBar.Render("x")) + lipgloss.Height(m.list.Styles.StatusBar.Render("x"))
	d := list.NewDefaultDelegate()
	row := y - top
	if row < 0 || row%(d.Height()+d.Spacing()) >= d.Height() {
		return -1
	}
	i := m.list.Paginator.Page*m.list.Paginator.PerPage + row/(d.Height()+d.Spacing())
	if i >= len(m.list.VisibleItems()) {
		return -1
	}
	return i
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height-1)
		m.vp.Width, m.vp.Height = msg.Width, msg.Height-1
		if m.detail {
			m.setDetail()
		}
		return m, nil
	case statusMsg:
		m.status = string(msg)
		return m, nil
	case resultsMsg:
		m.setResults(msg)
		m.status = ""
		return m, nil
	case tea.MouseMsg:
		if m.detail || m.searching || msg.Action != tea.MouseActionPress {
			break
		}
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.list.CursorUp()
		case tea.MouseButtonWheelDown:
			m.list.CursorDown()
		case tea.MouseButtonLeft:
			if i := m.itemAt(msg.Y); i >= 0 {
				m.list.Select(i)
				m.detail = true
				m.setDetail()
			}
		}
		return m, nil
	case tea.KeyMsg:
		if m.searching {
			switch msg.Type {
			case tea.KeyEnter:
				if strings.TrimSpace(m.in.Value()) == "" {
					return m, nil
				}
				m.searching = false
				m.status = "searching…"
				return m, m.search(m.in.Value(), sLimit, false)
			case tea.KeyCtrlC:
				return m, tea.Quit
			case tea.KeyEsc:
				if m.raw == "" { // nothing to go back to
					return m, tea.Quit
				}
				m.searching = false
				return m, nil
			}
			var cmd tea.Cmd
			m.in, cmd = m.in.Update(msg)
			return m, cmd
		}
		if m.list.FilterState() == list.Filtering {
			break
		}
		r := m.selected()
		switch {
		case r != nil && key.Matches(msg, keyPDF):
			m.status = "downloading " + shortID(r) + "…"
			return m, m.fetchPDF(r, false)
		case r != nil && key.Matches(msg, keyOpen):
			m.status = "opening " + shortID(r) + "…"
			return m, m.fetchPDF(r, true)
		case r != nil && key.Matches(msg, keyWeb):
			m.status = openURL("https://europepmc.org/article/" + r.Source + "/" + r.ID)
			return m, nil
		case m.detail && key.Matches(msg, keyBack):
			m.detail = false
			return m, nil
		case m.detail:
			// fall through to the viewport
		case key.Matches(msg, keySearch):
			m.searching = true
			m.in.SetValue(m.raw)
			m.in.CursorEnd()
			return m, m.in.Focus()
		case key.Matches(msg, keyMore):
			if n := len(m.list.Items()); n < m.hits {
				m.status = "loading more…"
				return m, m.search(m.raw, n+sLimit, true)
			}
			m.status = "all results loaded"
			return m, nil
		case r != nil && key.Matches(msg, keyEnter):
			m.detail = true
			m.setDetail()
			return m, nil
		}
	}
	var cmd tea.Cmd
	if m.detail {
		m.vp, cmd = m.vp.Update(msg)
	} else {
		m.list, cmd = m.list.Update(msg)
	}
	return m, cmd
}

func (m *tuiModel) setDetail() {
	var b strings.Builder
	printRecord(&b, m.selected(), min(m.vp.Width-2, 100))
	m.vp.SetContent(b.String())
	m.vp.GotoTop()
}

func (m tuiModel) View() string {
	foot := m.status
	switch {
	case m.searching:
		return m.list.View() + "\n" + m.in.View()
	case m.detail:
		foot = stDim.Render(fmt.Sprintf("esc back · p pdf · o open pdf · b browser · ↑↓ scroll %3.0f%%  ", m.vp.ScrollPercent()*100)) + foot
		return m.vp.View() + "\n" + foot
	}
	return m.list.View() + "\n" + foot
}

// fetchPDF saves into cfg.PDFDir, like `gorxiv pdf`; with open it reuses an
// existing file and then launches cfg.Viewer on it.
func (m tuiModel) fetchPDF(r *epmc.Record, open bool) tea.Cmd {
	return func() tea.Msg {
		dst := filepath.Join(cfg.PDFDir, slug(shortID(r))+".pdf")
		msg := "have " + dst
		if _, err := os.Stat(dst); !open || err != nil {
			if err := os.MkdirAll(cfg.PDFDir, 0o755); err != nil {
				return statusMsg(err.Error())
			}
			s, _, err := m.c.FetchPDF(context.Background(), r, dst)
			if err != nil {
				return statusMsg("pdf " + shortID(r) + ": " + err.Error())
			}
			msg = fmt.Sprintf("saved %s (%d bytes)", s.Path, s.Size)
		}
		if open {
			args := append(strings.Fields(cfg.Viewer), dst)
			c := exec.Command(args[0], args[1:]...)
			if err := c.Start(); err != nil {
				return statusMsg("viewer: " + err.Error())
			}
			go c.Wait() // reap; the viewer outlives nothing but this goroutine
			msg += " · opened in " + args[0]
		}
		return statusMsg(msg)
	}
}

func openURL(u string) string {
	opener := "xdg-open"
	if runtime.GOOS == "darwin" {
		opener = "open"
	}
	if err := exec.Command(opener, u).Start(); err != nil {
		return "open: " + err.Error()
	}
	return "opened " + u
}
