import { create } from 'zustand'

export interface Position {
  zone: 'toolbar' | 'sidebar' | 'main' | 'footer'
  order: number
}

export interface ComponentSpec {
  id: string
  type: string
  label?: string
  pageId?: string
  props?: Record<string, unknown>
  position?: Position
  bindings?: Record<string, ActionHandler>
}

export interface ActionHandler {
  type: 'query' | 'modal' | 'nav' | 'filter'
  payload: string
}

export type UIEvent =
  | { op: 'add'; component: ComponentSpec }
  | { op: 'update'; id: string; props: Partial<ComponentSpec> }
  | { op: 'remove'; id: string }
  | { op: 'bind'; componentId: string; trigger: string; handler: ActionHandler }
  | { op: 'navigate'; id: string }

interface UIStore {
  components: Map<string, ComponentSpec>
  activePage: string
  setActivePage: (id: string) => void
  applyEvent: (evt: UIEvent) => void
  seedComponents: (components: ComponentSpec[]) => void
  reset: () => void
}

// Normalise the wire format: Go uses snake_case page_id
function normalise(c: any): ComponentSpec {
  return { ...c, pageId: c.pageId ?? c.page_id }
}

export const useUIStore = create<UIStore>((set) => ({
  components: new Map(),
  activePage: 'page-editor',

  setActivePage(id: string) {
    set({ activePage: id })
  },

  applyEvent(evt: UIEvent) {
    set((state) => {
      const next = new Map(state.components)
      switch (evt.op) {
        case 'add':
          next.set(evt.component.id, normalise(evt.component))
          return { components: next }
        case 'update': {
          const existing = next.get(evt.id)
          if (existing) {
            next.set(evt.id, {
              ...existing,
              ...evt.props,
              props: { ...(existing.props ?? {}), ...(evt.props.props ?? {}) },
            })
          }
          return { components: next }
        }
        case 'remove':
          next.delete(evt.id)
          return { components: next }
        case 'bind': {
          const comp = next.get(evt.componentId)
          if (comp) {
            next.set(evt.componentId, {
              ...comp,
              bindings: { ...(comp.bindings ?? {}), [evt.trigger]: evt.handler },
            })
          }
          return { components: next }
        }
        case 'navigate':
          return { components: state.components, activePage: evt.id }
      }
    })
  },

  seedComponents(components: ComponentSpec[]) {
    set((state) => {
      const next = new Map(state.components)
      for (const c of components) {
        next.set(c.id, normalise(c))
      }
      return { components: next }
    })
  },

  reset() {
    set({ components: new Map(), activePage: 'page-editor' })
  },
}))
