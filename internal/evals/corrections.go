package evals

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
)

// Corrections is the survey self-check's reply (D-5.9n): a verdict for every
// item of the draft it checked, by id (a backlog item's key), with the whole
// corrected item for each one it replaces; new items; and new text for the
// fields that carry no id, by key path. applyCorrections applies it to the
// draft exactly, all or nothing, so the scored draft is the first draft plus
// the corrections the model chose, and nothing it merely reworded.
type Corrections struct {
	Verdicts []Verdict                  `json:"verdicts"`
	Add      []Addition                 `json:"add,omitempty"`
	Fields   map[string]json.RawMessage `json:"fields,omitempty"`
}

// Verdict is the self-check's finding on one item.
type Verdict struct {
	ID      string          `json:"id"`
	Verdict string          `json:"verdict"` // keep, replace, or remove
	Why     string          `json:"why,omitempty"`
	Item    json.RawMessage `json:"item,omitempty"` // replace: the whole item, same id
}

// Addition is a new item appended to a list of the draft.
type Addition struct {
	List string          `json:"list"` // the list's key path: risks, questions, modernisation.steps, items
	Why  string          `json:"why,omitempty"`
	Item json.RawMessage `json:"item"`
}

// fixedFields are the fields without an id the self-check may not set: the
// person's own words and the product's name are not its to correct.
var fixedFields = []string{"request", "product"}

// applyCorrections applies a corrections reply to a draft. It refuses the
// whole reply on any problem (an unknown or repeated id, an item without a
// verdict, a replacement that changes its id, an unknown field), returning
// every problem and no draft, so a correction that does not apply is never
// half-applied. changes lists what changed, as "E3 replaced".
func applyCorrections(draft, reply string) (corrected string, changes, problems []string) {
	add := func(f string, a ...any) { problems = append(problems, fmt.Sprintf(f, a...)) }
	root, err := decodeOrdered([]byte(draft))
	doc, ok := root.(*object)
	if err != nil || !ok {
		return "", nil, []string{"the draft is not a JSON object"}
	}
	var c Corrections
	dec := json.NewDecoder(strings.NewReader(draftOnly(reply)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return "", nil, []string{"the corrections are not one JSON object of verdicts, add, and fields: " + err.Error()}
	}
	if dec.More() {
		return "", nil, []string{"the corrections are followed by more text; reply with one JSON object"}
	}

	items, lists, fields := map[string]slot{}, map[string]*list{}, map[string]slot{}
	var order []string
	index(doc, "", items, lists, fields, &order, add)

	type op struct {
		at   slot
		item *object
	}
	var replace []op
	remove := map[*list][]int{}
	seen := map[string]bool{}
	for _, v := range c.Verdicts {
		at, known := items[v.ID]
		switch {
		case !known:
			add("verdict for %q: the draft has no item with that id", v.ID)
			continue
		case seen[v.ID]:
			add("item %s has more than one verdict", v.ID)
			continue
		}
		seen[v.ID] = true
		switch v.Verdict {
		case "keep", "remove":
			if len(v.Item) > 0 {
				add("item %s: a %s verdict carries no item", v.ID, v.Verdict)
			} else if v.Verdict == "remove" {
				if at.list == nil {
					add("item %s is not in a list, so it cannot be removed; replace it", v.ID)
				} else {
					remove[at.list] = append(remove[at.list], at.index)
				}
			}
		case "replace":
			o, err := itemObject(v.Item)
			if err != nil {
				add("item %s: the replacement is not a JSON object: %v", v.ID, err)
			} else if id, field := itemID(o); id != v.ID || field != at.field {
				add("item %s: the replacement must keep %s %q (it has %s %q)", v.ID, at.field, v.ID, field, id)
			} else {
				replace = append(replace, op{at, o})
			}
		default:
			add("item %s: verdict %q; want keep, replace, or remove", v.ID, v.Verdict)
		}
	}
	var missing []string
	for _, id := range order {
		if !seen[id] {
			missing = append(missing, id)
		}
	}
	if len(missing) > 12 {
		missing = append(missing[:12], fmt.Sprintf("and %d more", len(missing)-12))
	}
	if len(missing) > 0 {
		add("every item needs a verdict; none for %s", strings.Join(missing, ", "))
	}

	type addOp struct {
		to   *list
		item *object
		id   string
	}
	var adds []addOp
	for _, a := range c.Add {
		l := lists[a.List]
		o, err := itemObject(a.Item)
		id := ""
		if err == nil {
			id, _ = itemID(o)
		}
		switch {
		case l == nil:
			add("add to %q: the draft has no list of items at that key path", a.List)
		case err != nil:
			add("add to %s: the item is not a JSON object: %v", a.List, err)
		case id == "":
			add("add to %s: the item has no id", a.List)
		case items[id].field != "" || seen["add:"+id]:
			add("add to %s: id %s is already used", a.List, id)
		default:
			seen["add:"+id] = true
			adds = append(adds, addOp{l, o, id})
		}
	}
	type setOp struct {
		at  slot
		val any
	}
	var sets []setOp
	keys := make([]string, 0, len(c.Fields))
	for k := range c.Fields {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		at, ok := fields[k]
		v, err := decodeOrdered(c.Fields[k])
		switch {
		case slices.Contains(fixedFields, k):
			add("field %s is not the self-check's to change", k)
		case !ok:
			add("field %q: the draft has no field without an id at that key path (items change by verdict)", k)
		case err != nil:
			add("field %s: %v", k, err)
		case containsItem(v):
			add("field %s: a field's value holds no items; change items by verdict", k)
		default:
			sets = append(sets, setOp{at, v})
		}
	}
	if len(problems) > 0 {
		return "", nil, problems
	}

	// Everything checked; now apply, in an order that cannot fail.
	for _, r := range replace {
		before := encodeOrdered(r.at.get())
		r.at.set(r.item)
		if !bytes.Equal(before, encodeOrdered(r.item)) {
			changes = append(changes, r.at.id+" replaced")
		}
	}
	for l, idx := range remove {
		keep := l.elems[:0:0]
		for i, e := range l.elems {
			if !slices.Contains(idx, i) {
				keep = append(keep, e)
			}
		}
		l.elems = keep
	}
	for _, id := range order {
		if at := items[id]; at.list != nil && slices.Contains(remove[at.list], at.index) {
			changes = append(changes, id+" removed")
		}
	}
	for _, a := range adds {
		a.to.elems = append(a.to.elems, a.item)
		changes = append(changes, a.id+" added")
	}
	for i, s := range sets {
		if !bytes.Equal(encodeOrdered(s.at.get()), encodeOrdered(s.val)) {
			s.at.set(s.val)
			changes = append(changes, keys[i]+" set")
		}
	}
	return string(encodeOrdered(doc)), changes, nil
}

// slot is where a value sits: an element of a list or a key of an object.
type slot struct {
	id, field string // an item's id and whether it is "id" or "key"
	list      *list
	index     int
	obj       *object
	key       string
}

func (s slot) get() any {
	if s.list != nil {
		return s.list.elems[s.index]
	}
	return s.obj.vals[s.key]
}

func (s slot) set(v any) {
	if s.list != nil {
		s.list.elems[s.index] = v
	} else {
		s.obj.vals[s.key] = v
	}
}

// index records the draft's items (objects with an id, or a backlog item's
// key; their insides are the item's own), the lists that hold items or are
// empty, and the fields without an id: values reached through objects that
// hold no item.
func index(o *object, path string, items map[string]slot, lists map[string]*list, fields map[string]slot, order *[]string, add func(string, ...any)) {
	item := func(v any, at slot) bool {
		x, ok := v.(*object)
		if !ok {
			return false
		}
		id, field := itemID(x)
		if id == "" {
			return false
		}
		if _, dup := items[id]; dup {
			add("the draft has more than one item with id %s", id)
		}
		at.id, at.field = id, field
		items[id] = at
		*order = append(*order, id)
		return true
	}
	for _, k := range o.keys {
		p := k
		if path != "" {
			p = path + "." + k
		}
		switch v := o.vals[k].(type) {
		case *list:
			holds := false
			for i, e := range v.elems {
				holds = item(e, slot{list: v, index: i}) || holds
			}
			if holds || len(v.elems) == 0 {
				lists[p] = v
			}
			if !holds {
				fields[p] = slot{obj: o, key: k}
			}
		case *object:
			switch {
			case item(v, slot{obj: o, key: k}):
			case containsItem(v):
				index(v, p, items, lists, fields, order, add)
			default:
				fields[p] = slot{obj: o, key: k}
			}
		default:
			fields[p] = slot{obj: o, key: k}
		}
	}
}

// itemID is an object's id, or a backlog item's key, and which of the two.
func itemID(o *object) (id, field string) {
	for _, f := range []string{"id", "key"} {
		if s, ok := o.vals[f].(string); ok && s != "" {
			return s, f
		}
	}
	return "", ""
}

func containsItem(v any) bool {
	switch x := v.(type) {
	case *object:
		if id, _ := itemID(x); id != "" {
			return true
		}
		for _, k := range x.keys {
			if containsItem(x.vals[k]) {
				return true
			}
		}
	case *list:
		return slices.ContainsFunc(x.elems, containsItem)
	}
	return false
}

func itemObject(raw json.RawMessage) (*object, error) {
	v, err := decodeOrdered(raw)
	if err != nil {
		return nil, err
	}
	o, ok := v.(*object)
	if !ok {
		return nil, errors.New("not an object")
	}
	return o, nil
}

// object and list are JSON values that keep the order the model wrote, so a
// corrected draft reads like the draft it corrects.
type object struct {
	keys []string
	vals map[string]any
}

type list struct{ elems []any }

// decodeOrdered decodes one JSON value, keeping key order and numbers as written.
func decodeOrdered(b []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	v, err := decodeValue(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("more than one JSON value")
	}
	return v, nil
}

func decodeValue(dec *json.Decoder) (any, error) {
	t, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch d := t.(type) {
	case json.Delim:
		switch d {
		case '{':
			o := &object{vals: map[string]any{}}
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, err
				}
				k := kt.(string)
				v, err := decodeValue(dec)
				if err != nil {
					return nil, err
				}
				if _, dup := o.vals[k]; !dup {
					o.keys = append(o.keys, k)
				}
				o.vals[k] = v
			}
			_, err := dec.Token()
			return o, err
		case '[':
			l := &list{}
			for dec.More() {
				v, err := decodeValue(dec)
				if err != nil {
					return nil, err
				}
				l.elems = append(l.elems, v)
			}
			_, err := dec.Token()
			return l, err
		}
		return nil, fmt.Errorf("unexpected %v", d)
	default:
		return t, nil
	}
}

// encodeOrdered writes a decoded value back as compact JSON, in its order.
func encodeOrdered(v any) []byte {
	var b bytes.Buffer
	writeOrdered(&b, v)
	return b.Bytes()
}

func writeOrdered(b *bytes.Buffer, v any) {
	switch x := v.(type) {
	case *object:
		b.WriteByte('{')
		for i, k := range x.keys {
			if i > 0 {
				b.WriteByte(',')
			}
			writeScalar(b, k)
			b.WriteByte(':')
			writeOrdered(b, x.vals[k])
		}
		b.WriteByte('}')
	case *list:
		b.WriteByte('[')
		for i, e := range x.elems {
			if i > 0 {
				b.WriteByte(',')
			}
			writeOrdered(b, e)
		}
		b.WriteByte(']')
	default:
		writeScalar(b, x)
	}
}

func writeScalar(b *bytes.Buffer, v any) {
	enc := json.NewEncoder(b)
	enc.SetEscapeHTML(false) // keep "<?maruku" as written
	_ = enc.Encode(v)
	b.Truncate(b.Len() - 1) // Encode ends with a newline
}
