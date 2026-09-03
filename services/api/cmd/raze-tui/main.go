package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/aksayush2005/raze/services/api/internal/models"
)

// keyEvent is one decoded input: either a printable rune (r) or a named key.
type keyEvent struct {
	r    rune
	name string // "up","down","left","right","pgup","pgdown","enter","esc",...
}

// app holds the interactive UI state.
type app struct {
	c      *client
	api    string
	actor  string
	keys   chan keyEvent
	quit   bool
	help   bool
	ticker *time.Ticker

	view   string // "jobs" | "job" | "item" | "records"
	jobID  int64
	itemID int64
	sel    int
	scroll int
	filter string // item status filter on the job view

	jobs    []*models.Job
	job     *models.Job
	items   []*models.ItemView
	detail  *itemDetail
	records []*models.Record

	msg     string
	err     string
	lastRef time.Time

	// inline input prompt (manual-link target id).
	prompt    bool
	promptLbl string
	promptBuf []rune
	promptCb  func(string)

	// preKey, when set, is the first key pressed during the startup splash; it
	// is handled as soon as the main loop starts so e.g. 'q' still quits.
	preKey *keyEvent
}

func main() {
	api := flag.String("api", envOr("RAZE_API_URL", "http://localhost:8080"), "control-plane base URL")
	actor := flag.String("actor", envOr("RAZE_ACTOR", "operator@tui"), "operator identity for review actions")
	once := flag.Bool("once", false, "render the current view once as plain text and exit")
	watch := flag.Bool("watch", false, "re-render the current view on an interval")
	interval := flag.Duration("interval", 2*time.Second, "refresh interval")
	view := flag.String("view", "jobs", "view: jobs | records | job=<id> | item=<id>")
	importFile := flag.String("import", "", "import records from this JSON file before starting")
	flag.Parse()

	c := newClient(*api)

	if *importFile != "" {
		recs, err := loadRecords(*importFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "raze-tui:", err)
			os.Exit(1)
		}
		n, err := c.ImportRecords(context.Background(), recs)
		if err != nil {
			fmt.Fprintln(os.Stderr, "raze-tui: import:", err)
			os.Exit(1)
		}
		fmt.Printf("imported %d records from %s\n", n, *importFile)
	}

	// Non-interactive modes. A non-TTY stdin also degrades to --once so piping
	// keys never triggers raw mode.
	if *once || *watch || !isTTY(os.Stdin) {
		useColor = false
		runPlain(c, *view, *watch, *interval, *actor)
		return
	}

	a := newApp(c, *api, *actor)
	a.runInteractive()
}

// newApp wires the interactive app and starts the key reader.
func newApp(c *client, api, actor string) *app {
	return &app{
		c:      c,
		api:    api,
		actor:  actor,
		keys:   keyReader(),
		ticker: time.NewTicker(2 * time.Second),
		view:   "jobs",
		sel:    -1,
	}
}

/* ---------------- terminal plumbing ---------------- */

var savedStty string

func stty(args ...string) (string, error) {
	cmd := exec.Command("stty", args...)
	cmd.Stdin = os.Stdin
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func enterRaw() error {
	saved, err := stty("-g")
	if err != nil {
		return fmt.Errorf("stty -g: %w", err)
	}
	savedStty = strings.TrimSpace(saved)
	if _, err := stty("-icanon", "-echo", "min", "1", "time", "0"); err != nil {
		return fmt.Errorf("stty raw: %w", err)
	}
	fmt.Print("\x1b[?1049h\x1b[?25l") // alternate screen + hide cursor
	return nil
}

func restoreTerm() {
	fmt.Print("\x1b[?25h\x1b[?1049l\x1b[0m") // show cursor + leave alt screen
	if savedStty != "" {
		_, _ = stty(savedStty)
	}
}

func termSize() (rows, cols int) {
	rows, cols = 24, 80
	if out, err := stty("size"); err == nil {
		var r, c int
		if _, e := fmt.Sscanf(out, "%d %d", &r, &c); e == nil && r > 0 && c > 0 {
			return r, c
		}
	}
	if v := os.Getenv("LINES"); v != "" {
		if n, e := strconv.Atoi(v); e == nil && n > 0 {
			rows = n
		}
	}
	if v := os.Getenv("COLUMNS"); v != "" {
		if n, e := strconv.Atoi(v); e == nil && n > 0 {
			cols = n
		}
	}
	return rows, cols
}

func isTTY(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// waitReadable polls fd for readability up to ms milliseconds (used to
// disambiguate a bare Esc from the start of an escape sequence).
func waitReadable(fd uintptr, ms int) bool {
	var rfds syscall.FdSet
	rfds.Bits[fd/64] |= 1 << (fd % 64)
	tv := syscall.Timeval{Sec: 0, Usec: int64(ms) * 1000}
	n, err := syscall.Select(int(fd)+1, &rfds, nil, nil, &tv)
	return err == nil && n > 0
}

// keyReader decodes raw terminal input into keyEvents.
func keyReader() chan keyEvent {
	ch := make(chan keyEvent, 32)
	go func() {
		fd := os.Stdin.Fd()
		one := make([]byte, 1)
		for {
			n, err := os.Stdin.Read(one)
			if err != nil || n == 0 {
				return
			}
			b := one[0]
			if b != 0x1b {
				ch <- keyEvent{r: rune(b)}
				continue
			}
			// Escape sequence: peek up to 3 more bytes to distinguish arrows
			// and function keys from a bare Esc.
			seq := []byte{0x1b}
			for len(seq) < 4 && waitReadable(fd, 20) {
				n, _ := os.Stdin.Read(one)
				if n == 0 {
					break
				}
				seq = append(seq, one[0])
			}
			switch string(seq) {
			case "\x1b[A":
				ch <- keyEvent{name: "up"}
			case "\x1b[B":
				ch <- keyEvent{name: "down"}
			case "\x1b[C":
				ch <- keyEvent{name: "right"}
			case "\x1b[D":
				ch <- keyEvent{name: "left"}
			case "\x1b[5~":
				ch <- keyEvent{name: "pgup"}
			case "\x1b[6~":
				ch <- keyEvent{name: "pgdown"}
			case "\x1bOH", "\x1b[1~":
				ch <- keyEvent{name: "home"}
			case "\x1bOF", "\x1b[4~":
				ch <- keyEvent{name: "end"}
			default:
				if len(seq) == 1 {
					ch <- keyEvent{name: "esc"}
				}
			}
		}
	}()
	return ch
}

/* ---------------- main loop ---------------- */

func (a *app) runInteractive() {
	if err := enterRaw(); err != nil {
		fmt.Fprintln(os.Stderr, "raze-tui: cannot set raw mode:", err)
		os.Exit(1)
	}
	defer restoreTerm()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sig)

	a.playIntro() // animated RAZE splash; any key skips

	a.refresh()
	a.draw()
	if a.preKey != nil {
		k := *a.preKey
		a.preKey = nil
		a.handleKey(k)
		a.draw()
	}
	for !a.quit {
		select {
		case <-sig:
			a.quit = true
		case <-a.ticker.C:
			a.refresh()
			a.draw()
		case k := <-a.keys:
			a.handleKey(k)
			a.draw()
		}
	}
	restoreTerm()
}

/* ---------------- refresh ---------------- */

func (a *app) refresh() {
	ctx := context.Background()
	a.lastRef = time.Now()
	a.err = ""
	switch a.view {
	case "jobs":
		jobs, err := a.c.ListJobs(ctx, 50)
		if err != nil {
			a.err = err.Error()
			return
		}
		a.jobs = jobs
		if a.sel >= len(a.jobs) {
			a.sel = len(a.jobs) - 1
		}
	case "job":
		job, err := a.c.GetJob(ctx, a.jobID)
		if err != nil {
			a.err = err.Error()
			return
		}
		a.job = job
		items, err := a.c.ListJobItems(ctx, a.jobID, a.filter, 500)
		if err != nil {
			a.err = err.Error()
			return
		}
		a.items = items
		if a.sel >= len(a.items) {
			a.sel = len(a.items) - 1
		}
	case "item":
		d, err := a.c.GetItem(ctx, a.itemID)
		if err != nil {
			a.err = err.Error()
			return
		}
		a.detail = d
	case "records":
		recs, err := a.c.ListRecords(ctx, 200)
		if err != nil {
			a.err = err.Error()
			return
		}
		a.records = recs
	}
}

// gotoView switches the view and refreshes immediately.
func (a *app) gotoView(view string, jobID, itemID int64) {
	a.view = view
	a.jobID = jobID
	a.itemID = itemID
	a.sel = 0
	a.scroll = 0
	a.refresh()
}

func (a *app) back() {
	switch a.view {
	case "item":
		a.gotoView("job", a.jobID, 0)
	case "job", "records":
		a.gotoView("jobs", 0, 0)
	}
}

/* ---------------- input ---------------- */

func (a *app) handleKey(k keyEvent) {
	if a.prompt {
		a.handlePrompt(k)
		return
	}
	switch k.r {
	case 'q', 0x03: // q or Ctrl-C
		a.quit = true
		return
	case 'r':
		a.refresh()
		return
	case '?':
		a.help = !a.help
		return
	}
	// b and Esc both go back (b is what the footer keymaps advertise). A
	// printable 'b' only reaches here when no prompt is open — while typing in a
	// prompt, handlePrompt above consumes it as text.
	if k.name == "esc" || k.r == 'b' {
		a.help = false
		a.back()
		return
	}
	switch a.view {
	case "jobs":
		a.keyJobs(k)
	case "job":
		a.keyJob(k)
	case "item":
		a.keyItem(k)
	case "records":
		a.keyRecords(k)
	}
}

func (a *app) moveSel(delta int) {
	n := 0
	switch a.view {
	case "jobs":
		n = len(a.jobs)
	case "job":
		n = len(a.items)
	case "records":
		n = len(a.records)
	}
	if n == 0 {
		return
	}
	a.sel += delta
	if a.sel < 0 {
		a.sel = 0
	}
	if a.sel >= n {
		a.sel = n - 1
	}
}

// isEnter reports whether k is an Enter press. Raw mode via stty keeps ICRNL
// enabled by default, so the terminal driver may deliver Enter as either \r or
// \n — accept both.
func isEnter(k keyEvent) bool {
	return k.r == '\r' || k.r == '\n' || k.name == "enter"
}

func (a *app) keyJobs(k keyEvent) {
	switch {
	case k.name == "up" || k.r == 'k':
		a.moveSel(-1)
	case k.name == "down" || k.r == 'j':
		a.moveSel(1)
	case k.name == "pgup":
		a.moveSel(-10)
	case k.name == "pgdown":
		a.moveSel(10)
	case isEnter(k):
		if a.sel >= 0 && a.sel < len(a.jobs) {
			a.gotoView("job", a.jobs[a.sel].ID, 0)
		}
	case k.r == 'n':
		job, err := a.c.CreateJob(context.Background(), "reconciliation", map[string]any{})
		if err != nil {
			a.err = err.Error()
			return
		}
		a.msg = fmt.Sprintf("created job #%d — reconciling", job.ID)
		a.gotoView("job", job.ID, 0)
	case k.r == 'i':
		recs, err := loadRecords(envOr("RAZE_IMPORT_FILE", "data/benchmark/records.json"))
		if err != nil {
			a.err = "import: " + err.Error()
			return
		}
		n, err := a.c.ImportRecords(context.Background(), recs)
		if err != nil {
			a.err = "import: " + err.Error()
			return
		}
		a.msg = fmt.Sprintf("imported %d records", n)
		a.refresh()
	case k.r == 'v':
		a.gotoView("records", 0, 0)
	}
}

func (a *app) keyJob(k keyEvent) {
	switch {
	case k.name == "up" || k.r == 'k':
		a.moveSel(-1)
	case k.name == "down" || k.r == 'j':
		a.moveSel(1)
	case k.name == "pgup":
		a.moveSel(-10)
	case k.name == "pgdown":
		a.moveSel(10)
	case isEnter(k):
		if a.sel >= 0 && a.sel < len(a.items) {
			a.gotoView("item", a.jobID, a.items[a.sel].ID)
		}
	case k.r == 'f':
		switch a.filter {
		case "":
			a.filter = "RESOLVED"
		case "RESOLVED":
			a.filter = "REVIEW"
		case "REVIEW":
			a.filter = "ESCALATED"
		default:
			a.filter = ""
		}
		a.sel = 0
		a.refresh()
	}
}

func (a *app) keyItem(k keyEvent) {
	switch {
	case k.name == "up" || k.r == 'k':
		a.scroll--
	case k.name == "down" || k.r == 'j':
		a.scroll++
	case k.name == "pgup":
		a.scroll -= 10
	case k.name == "pgdown":
		a.scroll += 10
	case a.reviewable() && k.r == '1':
		a.doReview("ACCEPTED_AGENT_MATCH", nil)
	case a.reviewable() && k.r == '2':
		a.doReview("REJECTED_CANDIDATE", nil)
	case a.reviewable() && k.r == '3':
		a.doReview("ESCALATED", nil)
	case a.reviewable() && k.r == '4':
		a.doReview("CONFIRMED_EXCEPTION", nil)
	case a.reviewable() && k.r == '5':
		a.beginPrompt("target record id for manual link:", func(v string) {
			id, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
			if err != nil {
				a.msg = "manual link cancelled — invalid id"
				return
			}
			a.doReview("MANUALLY_LINKED_RECORDS", &id)
		})
	}
}

func (a *app) keyRecords(k keyEvent) {
	switch {
	case k.name == "up" || k.r == 'k':
		a.moveSel(-1)
	case k.name == "down" || k.r == 'j':
		a.moveSel(1)
	case k.name == "pgup":
		a.moveSel(-10)
	case k.name == "pgdown":
		a.moveSel(10)
	}
}

func (a *app) reviewable() bool {
	return a.detail != nil && a.detail.Item != nil &&
		(a.detail.Item.Status == "REVIEW" || a.detail.Item.Status == "ESCALATED")
}

func (a *app) doReview(action string, target *int64) {
	item, err := a.c.ReviewItem(context.Background(), a.itemID, action, a.actor, target)
	if err != nil {
		a.err = err.Error()
		return
	}
	a.msg = fmt.Sprintf("review %s → %s", action, item.Status)
	a.refresh()
}

func (a *app) beginPrompt(label string, cb func(string)) {
	a.prompt = true
	a.promptLbl = label
	a.promptBuf = nil
	a.promptCb = cb
}

func (a *app) handlePrompt(k keyEvent) {
	switch {
	case k.name == "esc":
		a.prompt = false
		a.msg = "cancelled"
	case isEnter(k):
		val := string(a.promptBuf)
		a.prompt = false
		if a.promptCb != nil {
			a.promptCb(val)
		}
	case k.r == 0x7f || k.name == "backspace":
		if len(a.promptBuf) > 0 {
			a.promptBuf = a.promptBuf[:len(a.promptBuf)-1]
		}
	case k.r >= 0x20 && k.r <= 0x7e:
		a.promptBuf = append(a.promptBuf, k.r)
	}
}

/* ---------------- rendering ---------------- */

func (a *app) draw() {
	rows, cols := termSize()
	content := a.renderCurrent(cols)
	avail := rows - 2
	if avail < 1 {
		avail = 1
	}

	// Table views keep the selected row centered; the item view scrolls.
	if a.view == "item" {
		if a.scroll < 0 {
			a.scroll = 0
		}
		if len(content) > avail {
			if a.scroll > len(content)-avail {
				a.scroll = len(content) - avail
			}
			content = content[a.scroll : a.scroll+avail]
		}
	} else if len(content) > avail {
		top := a.sel - avail/2
		if top < 0 {
			top = 0
		}
		if top+avail > len(content) {
			top = len(content) - avail
		}
		content = content[top : top+avail]
	}
	for len(content) < avail {
		content = append(content, "")
	}

	right := fmt.Sprintf("%s · %s · %s", a.api, a.actor, a.lastRef.Format("15:04:05"))
	top := headerBar("RAZE · Reconciliation Control Plane", right, cols)
	bot := a.footer(cols)

	var sb strings.Builder
	sb.WriteString("\x1b[2J\x1b[H")
	sb.WriteString(top)
	sb.WriteString("\r\n")
	for _, l := range content {
		sb.WriteString(padTo(l, cols))
		sb.WriteString("\r\n")
	}
	sb.WriteString(bot)
	sb.WriteString("\r\n")
	fmt.Print(sb.String())
}

func (a *app) footer(width int) string {
	if a.prompt {
		return footerBar("enter confirm · esc cancel", a.promptLbl+" "+string(a.promptBuf)+"▌", width)
	}
	if a.err != "" {
		return footerBar("q quit", style("⚠ "+a.err, fg(cRed)), width)
	}
	return footerBar(a.hints(), a.msg, width)
}

func (a *app) hints() string {
	switch a.view {
	case "jobs":
		return "↑↓ select · enter open · n new job · i import · v records · r refresh · ? help · q quit"
	case "job":
		return "↑↓ select · enter open · f filter:" + filterLabel(a.filter) + " · b back · ? help · q quit"
	case "item":
		if a.reviewable() {
			return "1 accept · 2 reject · 3 escalate · 4 confirm · 5 manual link · ↑↓ scroll · b back · q quit"
		}
		return "↑↓ scroll · b back · q quit"
	case "records":
		return "↑↓ scroll · b back · q quit"
	}
	return "q quit"
}

func filterLabel(f string) string {
	if f == "" {
		return "ALL"
	}
	return f
}

func (a *app) renderCurrent(w int) []string {
	if a.help {
		return helpLines()
	}
	switch a.view {
	case "jobs":
		return a.renderJobs(w)
	case "job":
		return a.renderJob(w)
	case "item":
		return a.renderItem(w)
	case "records":
		return a.renderRecords(w)
	}
	return []string{"unknown view"}
}

func (a *app) renderJobs(w int) []string {
	headers := []string{"ID", "NAME", "STATUS", "RECORDS", "MATCHED", "REVIEW", "ESCALATED", "UPDATED"}
	right := []bool{true, false, false, true, true, true, true, false}
	rows := [][]string{}
	for _, j := range a.jobs {
		rows = append(rows, []string{
			fmt.Sprintf("#%d", j.ID),
			j.Name,
			j.Status,
			strconv.FormatInt(j.TotalRecords, 10),
			strconv.FormatInt(j.Matched, 10),
			strconv.FormatInt(j.Review, 10),
			strconv.FormatInt(j.Escalated, 10),
			formatTime(j.UpdatedAt),
		})
	}
	lines := tableLines(headers, right, rows, a.sel, 2, w)
	if len(a.jobs) == 0 {
		lines = append(lines, style("  no jobs yet — press n to run a reconciliation", fg(cGray)))
	}
	// Totals strip.
	var totRec, totMat, totRev, totEsc int64
	for _, j := range a.jobs {
		totRec += j.TotalRecords
		totMat += j.Matched
		totRev += j.Review
		totEsc += j.Escalated
	}
	if len(a.jobs) > 0 {
		lines = append(lines, sectionHeader(fmt.Sprintf("totals  %d records · %d matched · %d review · %d escalated", totRec, totMat, totRev, totEsc), w))
	}
	return lines
}

func (a *app) renderJob(w int) []string {
	var lines []string
	j := a.job
	if j == nil {
		return []string{style("  job not loaded", fg(cGray))}
	}
	stat := fmt.Sprintf("  %s   records %d   matched %d   review %d   escalated %d",
		pill(j.Status), j.TotalRecords, j.Matched, j.Review, j.Escalated)
	lines = append(lines, style("Job #"+strconv.FormatInt(j.ID, 10)+" — "+j.Name, colBold))
	lines = append(lines, stat)
	lines = append(lines, style("  filter: "+filterLabel(a.filter)+"   ·   "+a.lastRef.Format("15:04:05"), fg(cGray)))
	lines = append(lines, "")

	headers := []string{"ITEM", "RECORD", "KIND", "AMOUNT", "STATUS", "CONF", "DECISION"}
	right := []bool{true, false, false, true, false, true, false}
	rows := [][]string{}
	for _, it := range a.items {
		conf := "—"
		if it.Confidence != nil {
			conf = fmt.Sprintf("%.0f%%", *it.Confidence*100)
		}
		dec := "—"
		if it.Decision != nil {
			dec = *it.Decision
		}
		rows = append(rows, []string{
			fmt.Sprintf("#%d", it.ID),
			it.RecordExternalID,
			it.RecordKind,
			formatMoney(it.AmountMinor),
			it.Status,
			conf,
			dec,
		})
	}
	lines = append(lines, tableLines(headers, right, rows, a.sel, 4, w)...)
	if len(a.items) == 0 {
		lines = append(lines, style("  no items"+"  (filter: "+filterLabel(a.filter)+")", fg(cGray)))
	}
	return lines
}

func amt(rec *models.Record) (int64, int64, int64, int64) {
	if rec == nil {
		return 0, 0, 0, 0
	}
	return rec.AmountMinor, rec.FeeMinor, rec.TaxMinor, rec.NetMinor
}

func (a *app) renderItem(w int) []string {
	d := a.detail
	if d == nil || d.Item == nil {
		return []string{style("  item not loaded", fg(cGray))}
	}
	it := d.Item
	rec := d.Record
	var lines []string

	tag := "provider"
	if rec != nil && rec.IsSynthetic {
		tag = "synthetic"
	}
	kind, ext := "", ""
	if rec != nil {
		kind, ext = rec.Kind, rec.ExternalID
	}
	lines = append(lines, style(fmt.Sprintf("Case #%d  %s", it.ID, pill(it.Status)), colBold))
	lines = append(lines, style(fmt.Sprintf("  job #%d · record #%d · item v%d", it.JobID, it.RecordID, it.Version), fg(cGray)))
	lines = append(lines, "")

	amt, fee, tax, net := amt(rec)
	cur := "INR"
	if rec != nil {
		cur = rec.Currency
	}
	occ := "—"
	if rec != nil {
		occ = formatTime(rec.OccurredAt)
	}
	conf := "—"
	if it.Confidence != nil {
		conf = formatPct(it.Confidence)
	}
	dec := "—"
	if it.Decision != nil {
		dec = *it.Decision
	}
	kv := [][2]string{
		{"Record", fmt.Sprintf("%s  (%s · %s)", ext, kind, tag)},
		{"Amount", fmt.Sprintf("%s  %s", formatMoney(amt), cur)},
		{"Fee / Tax / Net", fmt.Sprintf("%s / %s / %s", formatMoney(fee), formatMoney(tax), formatMoney(net))},
		{"Occurred", occ},
	}
	if d.MatchRecord != nil {
		kv = append(kv, [2]string{"Matched", fmt.Sprintf("%s  %s", d.MatchRecord.ExternalID, formatMoney(d.MatchRecord.AmountMinor))})
	}
	kv = append(kv, [2]string{"Confidence", conf})
	kv = append(kv, [2]string{"Decision", dec})
	for _, p := range kv {
		lines = append(lines, "  "+style(padTo(p[0], 16), fg(cGray))+style(p[1], colBold))
	}

	if len(d.Candidates) > 0 {
		lines = append(lines, sectionHeader("Candidates", w))
		for _, cand := range d.Candidates {
			lines = append(lines, fmt.Sprintf("  %s  %s  sim=%s  %s",
				style(fmt.Sprintf("#%d", cand.TargetRecordID), fg(cCyan)),
				cand.Strategy,
				fmt.Sprintf("%.0f%%", cand.Similarity*100),
				pill(cand.Status)))
		}
	}

	if len(d.Evidence) > 0 {
		lines = append(lines, sectionHeader("Evidence", w))
		for _, ev := range d.Evidence {
			weight := fmt.Sprintf("%+.2f", ev.Weight)
			det := "{}"
			if b, err := json.Marshal(ev.Details); err == nil && len(b) > 2 {
				det = string(b)
			}
			lines = append(lines, fmt.Sprintf("  %s   %s",
				style(ev.Type, fg(cGreen)), style(weight, colBold)+"  "+style(truncate(det, 60), fg(cGray))))
		}
	}

	lines = append(lines, sectionHeader("AI investigation · advisory", w))
	if len(d.AIDecisions) > 0 {
		for _, ai := range d.AIDecisions {
			lines = append(lines, fmt.Sprintf("  %s  %s  %s",
				style(ai.Recommendation, fg(cYellow)),
				formatPct(&ai.Confidence),
				style("model="+ai.ModelVersion, fg(cGray))))
			// The narrative lives in investigation.summary (LLM prose or
			// heuristic notes). Render it word-wrapped so it is fully readable
			// via the item view's ↑↓ scroll — never truncate it away.
			summary, _ := ai.Investigation["summary"].(string)
			if summary == "" {
				if b, err := json.Marshal(ai.Investigation); err == nil && len(b) > 2 {
					summary = string(b)
				}
			}
			for _, ln := range wrapText(summary, w-2) {
				lines = append(lines, "  "+style(ln, fg(cGray)))
			}
			if uev, ok := ai.Investigation["unexplained_evidence"].([]any); ok && len(uev) > 0 {
				parts := make([]string, 0, len(uev))
				for _, u := range uev {
					parts = append(parts, fmt.Sprintf("%v", u))
				}
				lines = append(lines, "  "+style("unexplained: "+strings.Join(parts, ", "), fg(cYellow)))
			}
		}
	} else {
		lines = append(lines, style("  no AI investigation run (deterministic path)", fg(cGray)))
	}

	lines = append(lines, sectionHeader("Audit trail", w))
	if len(d.Audit) > 0 {
		for _, e := range d.Audit {
			lines = append(lines, fmt.Sprintf("  %s  %s  %s",
				style(formatTime(e.CreatedAt), fg(cGray)),
				style(e.Action, colBold),
				style("by "+e.Actor, fg(cGray))))
		}
	} else {
		lines = append(lines, style("  no audit events", fg(cGray)))
	}
	return lines
}

func (a *app) renderRecords(w int) []string {
	headers := []string{"ID", "EXTERNAL", "KIND", "AMOUNT", "FEE", "TAX", "NET", "REF", "SOURCE"}
	right := []bool{true, false, false, true, true, true, true, false, false}
	rows := [][]string{}
	for _, r := range a.records {
		ref := ""
		if r.RefExternalID != nil {
			ref = *r.RefExternalID
		}
		src := "provider"
		if r.IsSynthetic {
			src = "synthetic"
		}
		rows = append(rows, []string{
			strconv.FormatInt(r.ID, 10),
			r.ExternalID,
			r.Kind,
			formatMoney(r.AmountMinor),
			formatMoney(r.FeeMinor),
			formatMoney(r.TaxMinor),
			formatMoney(r.NetMinor),
			ref,
			src,
		})
	}
	lines := tableLines(headers, right, rows, a.sel, -1, w)
	if len(a.records) == 0 {
		lines = append(lines, style("  no records — import a dataset first (jobs view · i)", fg(cGray)))
	}
	return lines
}

func helpLines() []string {
	return []string{
		"",
		style("  RAZE terminal UI — keymap", colBold+fg(cCyan)),
		"",
		"  global        q / Ctrl-C   quit   ·   r refresh   ·   ? toggle help",
		"  jobs view     ↑↓ / j k     select   ·   enter  open job",
		"                n  create a reconciliation run",
		"                i  import data/benchmark/records.json",
		"                v  records view",
		"  job view      ↑↓ / j k     select item   ·   enter  open item",
		"                f  cycle filter (ALL → RESOLVED → REVIEW → ESCALATED)",
		"  item view     ↑↓ / j k     scroll",
		"                review (when REVIEW / ESCALATED):",
		"                  1 accept match · 2 reject candidate · 3 escalate",
		"                  4 confirm exception · 5 manual link (enter target id)",
		"  records view  ↑↓ / j k     scroll",
		"  back          b / esc",
		"",
		style("  modes:  --once <view>   one plain-text snapshot ·  --watch   live", fg(cGray)),
		style("          --view jobs|records|job=<id>|item=<id>   ·   --api <url>   ·   --actor <id>", fg(cGray)),
		"",
	}
}

/* ---------------- non-interactive modes ---------------- */

func runPlain(c *client, view string, watch bool, interval time.Duration, actor string) {
	a := &app{c: c, api: c.baseURL, actor: actor, view: "jobs", sel: -1}
	switch {
	case view == "records":
		a.view = "records"
	case strings.HasPrefix(view, "job="):
		a.view = "job"
		a.jobID = parseID(view[4:])
	case strings.HasPrefix(view, "item="):
		a.view = "item"
		a.itemID = parseID(view[5:])
	}
	first := true
	ttyOut := isTTY(os.Stdout)
	for {
		a.refresh()
		if a.err != "" {
			fmt.Fprintln(os.Stderr, "raze-tui:", a.err)
			os.Exit(1)
		}
		lines := a.renderCurrent(120)
		if !first && ttyOut {
			fmt.Print("\x1b[2J\x1b[H")
		} else if !first {
			fmt.Println(strings.Repeat("─", 48))
		}
		fmt.Println(strings.Join(lines, "\n"))
		first = false
		if !watch {
			return
		}
		time.Sleep(interval)
	}
}

func parseID(s string) int64 {
	id, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return id
}

/* ---------------- helpers ---------------- */

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func loadRecords(path string) ([]*models.Record, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var req importReq
	if err := json.Unmarshal(b, &req); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return req.Records, nil
}
