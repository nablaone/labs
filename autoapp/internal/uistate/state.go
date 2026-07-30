package uistate

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

type ComponentSpec struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Label    string         `json:"label,omitempty"`
	PageID   string         `json:"page_id,omitempty"`
	Props    map[string]any `json:"props,omitempty"`
	Position *Position      `json:"position,omitempty"`
}

type Position struct {
	Zone  string `json:"zone"`
	Order int    `json:"order"`
}

type UIEvent struct {
	Op        string         `json:"op"`                  // "add" | "update" | "remove" | "bind" | "navigate"
	Component *ComponentSpec `json:"component,omitempty"` // for "add"
	ID        string         `json:"id,omitempty"`        // for "update" | "remove" | "bind" | "navigate"
	Props     map[string]any `json:"props,omitempty"`     // for "update"
	Trigger   string         `json:"trigger,omitempty"`   // for "bind"
	Handler   map[string]any `json:"handler,omitempty"`   // for "bind"
}

type Manager struct {
	mu         sync.RWMutex
	components map[string]*ComponentSpec
	counter    atomic.Int64
	filePath   string
}

// LoadOrNew loads UI state from filePath if it exists, otherwise returns an empty manager.
func LoadOrNew(filePath string) *Manager {
	m := &Manager{
		components: make(map[string]*ComponentSpec),
		filePath:   filePath,
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return m
	}
	var specs []ComponentSpec
	if err := json.Unmarshal(data, &specs); err != nil {
		log.Printf("[uistate] failed to parse %s: %v — starting fresh", filePath, err)
		return m
	}
	for _, s := range specs {
		sc := s
		m.components[s.ID] = &sc
	}
	log.Printf("[uistate] loaded %d components from %s", len(specs), filePath)
	return m
}

func (m *Manager) save() {
	if m.filePath == "" {
		return
	}
	out := make([]ComponentSpec, 0, len(m.components))
	for _, c := range m.components {
		out = append(out, *c)
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		log.Printf("[uistate] marshal error: %v", err)
		return
	}
	if err := os.WriteFile(m.filePath, data, 0644); err != nil {
		log.Printf("[uistate] write error: %v", err)
	}
}

func (m *Manager) NextID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, m.counter.Add(1))
}

func (m *Manager) Add(spec ComponentSpec) (ComponentSpec, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if spec.ID == "" {
		return spec, fmt.Errorf("component ID required")
	}
	if _, exists := m.components[spec.ID]; exists {
		return spec, fmt.Errorf("component %s already exists", spec.ID)
	}
	m.components[spec.ID] = &spec
	m.save()
	return spec, nil
}

func (m *Manager) Update(id string, props map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	comp, ok := m.components[id]
	if !ok {
		return fmt.Errorf("component %s not found", id)
	}
	if comp.Props == nil {
		comp.Props = make(map[string]any)
	}
	for k, v := range props {
		comp.Props[k] = v
	}
	m.save()
	return nil
}

func (m *Manager) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.components[id]; !ok {
		return fmt.Errorf("component %s not found", id)
	}
	delete(m.components, id)
	m.save()
	return nil
}

func (m *Manager) Tree() []ComponentSpec {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]ComponentSpec, 0, len(m.components))
	for _, c := range m.components {
		out = append(out, *c)
	}
	return out
}

// DumpTree returns a human-readable multi-line string of the component tree grouped by page.
func (m *Manager) DumpTree() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pages := make([]*ComponentSpec, 0)
	byPage := make(map[string][]*ComponentSpec)

	for _, c := range m.components {
		if c.Type == "page" {
			pages = append(pages, c)
		} else {
			byPage[c.PageID] = append(byPage[c.PageID], c)
		}
	}

	sort.Slice(pages, func(i, j int) bool { return pages[i].ID < pages[j].ID })

	var sb strings.Builder
	fmt.Fprintf(&sb, "UI tree (%d pages, %d components):\n", len(pages), len(m.components)-len(pages))

	for _, pg := range pages {
		fmt.Fprintf(&sb, "  [page] %s — %q\n", pg.ID, pg.Label)
		children := byPage[pg.ID]
		sort.Slice(children, func(i, j int) bool {
			oi, oj := 0, 0
			if children[i].Position != nil {
				oi = children[i].Position.Order
			}
			if children[j].Position != nil {
				oj = children[j].Position.Order
			}
			return oi < oj
		})
		for _, c := range children {
			zone := ""
			if c.Position != nil {
				zone = fmt.Sprintf(" @%s/%d", c.Position.Zone, c.Position.Order)
			}
			fmt.Fprintf(&sb, "    %-14s %s%s — %q\n", c.Type, c.ID, zone, c.Label)
		}
	}

	if orphans := byPage[""]; len(orphans) > 0 {
		fmt.Fprintf(&sb, "  [unassigned]\n")
		for _, c := range orphans {
			fmt.Fprintf(&sb, "    %-14s %s — %q\n", c.Type, c.ID, c.Label)
		}
	}

	return sb.String()
}
