package brief

import (
	"context"
	"database/sql"
	"fmt"
	"html"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	_ "modernc.org/sqlite" // registers the "sqlite" driver (pure Go)
)

// ChartData is what a chart shows and how to get it again: the snapshot, its
// SHA-256, the query, and the rows it returned. Re-running Query on the
// snapshot reproduces Columns and Rows.
type ChartData struct {
	Figure   int      `json:"figure"`
	Source   string   `json:"source"` // the snapshot's source ID (D1, …)
	Snapshot string   `json:"snapshot"`
	SHA256   string   `json:"sha256"`
	Query    string   `json:"query"`
	Columns  []string `json:"columns"`
	Rows     [][]any  `json:"rows"`
	X        string   `json:"x"`
	Y        []string `json:"y"`
	Kind     string   `json:"kind"` // bar or line
}

const (
	maxChartRows    = 200
	maxChartColumns = 8
)

var (
	queryStart  = regexp.MustCompile(`(?is)^\s*(select|with)\b`)
	queryDenied = regexp.MustCompile(`(?i)\b(attach|detach|pragma|insert|update|delete|create|drop|alter|replace|vacuum|reindex|load_extension|readfile|writefile)\b`)
)

// CheckQuery accepts one read-only SELECT (or WITH … SELECT) statement.
func CheckQuery(q string) (string, error) {
	q = strings.TrimSuffix(strings.TrimSpace(q), ";")
	switch {
	case q == "":
		return "", fmt.Errorf("the chart has no query: put the SQL after the header lines")
	case !queryStart.MatchString(q):
		return "", fmt.Errorf("the query must be one SELECT (or WITH … SELECT) statement")
	case strings.Contains(q, ";"):
		return "", fmt.Errorf("the query must be one statement")
	case queryDenied.MatchString(q):
		return "", fmt.Errorf("the query uses %q; a chart query only reads", queryDenied.FindString(q))
	}
	return q, nil
}

// Query runs a read-only query on a snapshot and returns its columns and
// rows (integers, reals, and text; blobs as text).
func Query(ctx context.Context, snapshot, query string) ([]string, [][]any, error) {
	q, err := CheckQuery(query)
	if err != nil {
		return nil, nil, err
	}
	db, err := sql.Open("sqlite", "file:"+snapshot+"?mode=ro&_pragma=query_only(1)")
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = db.Close() }()
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, nil, fmt.Errorf("query: %w", err)
	}
	defer func() { _ = rows.Close() }()
	cols, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}
	if len(cols) > maxChartColumns {
		return nil, nil, fmt.Errorf("the query returns %d columns; a chart takes at most %d", len(cols), maxChartColumns)
	}
	var out [][]any
	for rows.Next() {
		if len(out) == maxChartRows {
			return nil, nil, fmt.Errorf("the query returns more than %d rows; aggregate it", maxChartRows)
		}
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, nil, err
		}
		for i, v := range vals {
			switch x := v.(type) {
			case []byte:
				vals[i] = string(x)
			case int64, float64, string, nil:
			default:
				vals[i] = fmt.Sprint(x)
			}
		}
		out = append(out, vals)
	}
	return cols, out, rows.Err()
}

func number(v any) (float64, bool) {
	switch x := v.(type) {
	case int64:
		return float64(x), true
	case float64:
		return x, true
	}
	return 0, false
}

func cell(v any) string {
	switch x := v.(type) {
	case nil:
		return "–"
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	}
	return fmt.Sprint(v)
}

// axes picks the x column and the y series: the named ones, else the first
// column and every numeric column after it.
func (c *ChartData) axes(x, y string) error {
	c.X = x
	if c.X == "" && len(c.Columns) > 0 {
		c.X = c.Columns[0]
	}
	if !slices.Contains(c.Columns, c.X) {
		return fmt.Errorf("x %q is not a column of the query (%s)", c.X, strings.Join(c.Columns, ", "))
	}
	if y != "" {
		for _, s := range strings.Split(y, ",") {
			c.Y = append(c.Y, strings.TrimSpace(s))
		}
	} else {
		for _, col := range c.Columns {
			if col != c.X {
				c.Y = append(c.Y, col)
			}
		}
	}
	if len(c.Y) == 0 {
		return fmt.Errorf("the chart has no series: the query needs a numeric column besides x")
	}
	for _, s := range c.Y {
		i := slices.Index(c.Columns, s)
		if i < 0 {
			return fmt.Errorf("y %q is not a column of the query (%s)", s, strings.Join(c.Columns, ", "))
		}
		for _, r := range c.Rows {
			if _, ok := number(r[i]); !ok && r[i] != nil {
				return fmt.Errorf("y %q has a value that is not a number (%v)", s, r[i])
			}
		}
	}
	if len(c.Rows) == 0 {
		return fmt.Errorf("the query returns no rows")
	}
	return nil
}

// Series colours (Okabe-Ito based, darkened to 3:1 or more on white).
var chartColours = []string{"#0b5fa5", "#b84a00", "#00745a", "#8e3f8a", "#6b5b00", "#3d4f63"}

// niceStep is a 1-2-5 step that splits span into about n ticks.
func niceStep(span float64, n int) float64 {
	if span <= 0 {
		return 1
	}
	raw := span / float64(n)
	mag := math.Pow(10, math.Floor(math.Log10(raw)))
	for _, m := range []float64{1, 2, 5, 10} {
		if raw <= m*mag {
			return m * mag
		}
	}
	return 10 * mag
}

// SVG renders the chart as an image named by alt, with a description that
// lists the data.
func (c *ChartData) SVG(id, alt string) string {
	const W, H, left, right, top, bottom = 640.0, 300.0, 64.0, 16.0, 36.0, 44.0
	e := html.EscapeString
	xi := slices.Index(c.Columns, c.X)
	yi := make([]int, len(c.Y))
	lo, hi := 0.0, 0.0
	for k, s := range c.Y {
		yi[k] = slices.Index(c.Columns, s)
		for _, r := range c.Rows {
			if v, ok := number(r[yi[k]]); ok {
				lo, hi = math.Min(lo, v), math.Max(hi, v)
			}
		}
	}
	step := niceStep(hi-lo, 5)
	lo, hi = math.Floor(lo/step)*step, math.Ceil(hi/step)*step
	if hi == lo {
		hi = lo + step
	}
	pw, ph := W-left-right, H-top-bottom
	yOf := func(v float64) float64 { return top + ph - (v-lo)/(hi-lo)*ph }
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %.0f %.0f" width="%.0f" height="%.0f" role="img" aria-labelledby="%s-title %s-desc" font-family="system-ui, -apple-system, 'Segoe UI', sans-serif">`, W, H, W, H, id, id)
	fmt.Fprintf(&b, `<title id="%s-title">%s</title><desc id="%s-desc">%s</desc>`, id, e(alt), id, e(c.describe()))
	for v := lo; v <= hi+step/2; v += step {
		y := yOf(v)
		fmt.Fprintf(&b, `<line x1="%.1f" x2="%.1f" y1="%.1f" y2="%.1f" stroke="#d9e2ec"/><text x="%.1f" y="%.1f" text-anchor="end" font-size="12" fill="#334e68">%s</text>`,
			left, W-right, y, y, left-8, y+4, e(strconv.FormatFloat(v, 'f', -1, 64)))
	}
	n := len(c.Rows)
	group := pw / float64(n)
	for i, r := range c.Rows {
		cx := left + group*(float64(i)+0.5)
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" text-anchor="middle" font-size="12" fill="#334e68">%s</text>`, cx, H-bottom+18, e(clip(cell(r[xi]), 14)))
	}
	fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" text-anchor="middle" font-size="12" fill="#334e68">%s</text>`, left+pw/2, H-6, e(c.X))
	labels := n*len(c.Y) <= 24
	for k := range c.Y {
		col := chartColours[k%len(chartColours)]
		if c.Kind == "line" {
			var pts []string
			for i, r := range c.Rows {
				if v, ok := number(r[yi[k]]); ok {
					pts = append(pts, fmt.Sprintf("%.1f,%.1f", left+group*(float64(i)+0.5), yOf(v)))
				}
			}
			fmt.Fprintf(&b, `<polyline points="%s" fill="none" stroke="%s" stroke-width="2.5"/>`, strings.Join(pts, " "), col)
			for i, r := range c.Rows {
				if v, ok := number(r[yi[k]]); ok {
					x, y := left+group*(float64(i)+0.5), yOf(v)
					fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="4" fill="%s"/>`, x, y, col)
					if labels {
						fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" text-anchor="middle" font-size="11" fill="#243b53">%s</text>`, x, y-8, e(cell(r[yi[k]])))
					}
				}
			}
			continue
		}
		bw := group * 0.72 / float64(len(c.Y))
		for i, r := range c.Rows {
			v, ok := number(r[yi[k]])
			if !ok {
				continue
			}
			x := left + group*float64(i) + group*0.14 + bw*float64(k)
			y0, y1 := yOf(math.Max(v, 0)), yOf(math.Min(v, 0))
			fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="%s"/>`, x, y0, bw, math.Max(y1-y0, 0.5), col)
			if labels {
				fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" text-anchor="middle" font-size="11" fill="#243b53">%s</text>`, x+bw/2, y0-4, e(cell(r[yi[k]])))
			}
		}
	}
	fmt.Fprintf(&b, `<line x1="%.1f" x2="%.1f" y1="%.1f" y2="%.1f" stroke="#52606d"/>`, left, W-right, yOf(0), yOf(0)) // the zero line
	lx := left
	for k, s := range c.Y { // legend
		fmt.Fprintf(&b, `<rect x="%.1f" y="10" width="12" height="12" fill="%s"/><text x="%.1f" y="20" font-size="12" fill="#243b53">%s</text>`,
			lx, chartColours[k%len(chartColours)], lx+16, e(s))
		lx += 16 + float64(len([]rune(s)))*7 + 20
	}
	b.WriteString(`</svg>`)
	return b.String()
}

// describe lists the chart's data in words, for its SVG description.
func (c *ChartData) describe() string {
	xi := slices.Index(c.Columns, c.X)
	var rows []string
	for _, r := range c.Rows {
		var vals []string
		for _, s := range c.Y {
			vals = append(vals, s+" "+cell(r[slices.Index(c.Columns, s)]))
		}
		rows = append(rows, c.X+" "+cell(r[xi])+": "+strings.Join(vals, ", "))
	}
	kind := "Bar chart"
	if c.Kind == "line" {
		kind = "Line chart"
	}
	return clip(fmt.Sprintf("%s of %s by %s. %s.", kind, strings.Join(c.Y, ", "), c.X, strings.Join(rows, "; ")), 1200)
}
