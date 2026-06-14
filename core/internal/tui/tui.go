// Package tui is rewynd's terminal UI: a live request list on the left, the selected request's
// full story (waterfall + queries + logs) on the right. A thin read client over the store.
package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/SrinjoyDev/rewynd/internal/model"
	"github.com/SrinjoyDev/rewynd/internal/store"
)

func Run(st *store.Store) error {
	p := tea.NewProgram(app{st: st}, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

type app struct {
	st            *store.Store
	reqs          []model.Request
	sel           int
	detail        *model.Request
	detailScroll  int
	width, height int
	filter        string // status class: "" | 2xx | 4xx | 5xx
	search        string // live path substring filter
	searching     bool   // typing into the search field
	slowOnly      bool
	help          bool
	err           error
}

type (
	reqsMsg   []model.Request
	detailMsg struct{ r *model.Request }
	tickMsg   struct{}
	errMsg    struct{ err error }
)

func (a app) listOpts() store.ListOptions {
	return store.ListOptions{StatusClass: a.filter, PathLike: a.search, Slow: a.slowOnly, Limit: 500}
}

func (a app) Init() tea.Cmd {
	return tea.Batch(loadReqs(a.st, a.listOpts()), tick())
}

func (a app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		return a, nil
	case tickMsg:
		return a, tea.Batch(loadReqs(a.st, a.listOpts()), tick())
	case reqsMsg:
		a.reqs = []model.Request(msg)
		if a.sel >= len(a.reqs) {
			a.sel = maxi(0, len(a.reqs)-1)
		}
		if len(a.reqs) > 0 {
			return a, loadDetail(a.st, a.reqs[a.sel].ID)
		}
		a.detail = nil
		return a, nil
	case detailMsg:
		a.detail = msg.r
		return a, nil
	case errMsg:
		a.err = msg.err
		return a, nil
	case tea.KeyMsg:
		return a.key(msg)
	}
	return a, nil
}

func (a app) key(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if a.help {
		if k.String() == "ctrl+c" {
			return a, tea.Quit
		}
		a.help = false
		return a, nil
	}
	if a.searching {
		return a.searchKey(k)
	}
	switch k.String() {
	case "?":
		a.help = true
		return a, nil
	case "q", "ctrl+c":
		return a, tea.Quit
	case "/":
		a.searching = true
		return a, nil
	case "esc":
		if a.search != "" {
			a.search = ""
			a.sel = 0
			return a, loadReqs(a.st, a.listOpts())
		}
		return a, nil
	case "j", "down":
		return a.move(1)
	case "k", "up":
		return a.move(-1)
	case "g":
		a.sel, a.detailScroll = 0, 0
		return a.selectSel()
	case "G":
		a.sel, a.detailScroll = maxi(0, len(a.reqs)-1), 0
		return a.selectSel()
	case "ctrl+d", "pgdown":
		return a.scrollDetail(a.halfPage())
	case "ctrl+u", "pgup":
		return a.scrollDetail(-a.halfPage())
	case "f":
		a.filter = nextFilter(a.filter)
		a.sel = 0
		return a, loadReqs(a.st, a.listOpts())
	case "s":
		a.slowOnly = !a.slowOnly
		a.sel = 0
		return a, loadReqs(a.st, a.listOpts())
	case "e":
		return a.nextError()
	case "c":
		return a, tea.Sequence(clearStore(a.st), loadReqs(a.st, a.listOpts()))
	}
	return a, nil
}

// searchKey routes keystrokes into the live path filter while the search field is open.
func (a app) searchKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "enter", "esc":
		a.searching = false
		if k.String() == "esc" {
			a.search = ""
			a.sel = 0
			return a, loadReqs(a.st, a.listOpts())
		}
		return a, nil
	case "ctrl+c":
		return a, tea.Quit
	case "backspace":
		if a.search != "" {
			a.search = a.search[:len(a.search)-1]
			a.sel = 0
			return a, loadReqs(a.st, a.listOpts())
		}
		return a, nil
	default:
		if k.Type == tea.KeyRunes {
			a.search += string(k.Runes)
			a.sel = 0
			return a, loadReqs(a.st, a.listOpts())
		}
	}
	return a, nil
}

func (a app) move(d int) (tea.Model, tea.Cmd) {
	if len(a.reqs) == 0 {
		return a, nil
	}
	a.sel = clampi(a.sel+d, 0, len(a.reqs)-1)
	a.detailScroll = 0
	return a.selectSel()
}

func (a app) selectSel() (tea.Model, tea.Cmd) {
	if len(a.reqs) == 0 {
		return a, nil
	}
	return a, loadDetail(a.st, a.reqs[a.sel].ID)
}

func (a app) scrollDetail(d int) (tea.Model, tea.Cmd) {
	if a.detail == nil {
		return a, nil
	}
	_, detailW, bodyH := a.layout()
	max := maxi(0, len(a.detailLines(a.detail, detailW))-bodyH)
	a.detailScroll = clampi(a.detailScroll+d, 0, max)
	return a, nil
}

func (a app) halfPage() int {
	_, _, bodyH := a.layout()
	return maxi(1, bodyH/2)
}

func (a app) nextError() (tea.Model, tea.Cmd) {
	for i := 1; i <= len(a.reqs); i++ {
		j := (a.sel + i) % len(a.reqs)
		if a.reqs[j].Error || a.reqs[j].StatusCode >= 500 {
			a.sel = j
			a.detailScroll = 0
			return a.selectSel()
		}
	}
	return a, nil
}

func loadReqs(st *store.Store, opts store.ListOptions) tea.Cmd {
	return func() tea.Msg {
		reqs, err := st.ListRequests(opts)
		if err != nil {
			return errMsg{err}
		}
		return reqsMsg(reqs)
	}
}

func loadDetail(st *store.Store, id string) tea.Cmd {
	return func() tea.Msg {
		r, err := st.GetRequest(id)
		if err != nil {
			return errMsg{err}
		}
		return detailMsg{r}
	}
}

func clearStore(st *store.Store) tea.Cmd {
	return func() tea.Msg { _ = st.Clear(); return nil }
}

func tick() tea.Cmd {
	return tea.Tick(700*time.Millisecond, func(time.Time) tea.Msg { return tickMsg{} })
}
