import { create } from 'zustand'

export interface QueryResult {
  columns: string[]
  rows: Record<string, unknown>[]
  error?: string
}

interface QueryStore {
  sql: string
  result: QueryResult | null
  loading: boolean
  history: string[]
  setSql: (sql: string) => void
  setResult: (r: QueryResult | null) => void
  setLoading: (v: boolean) => void
  pushHistory: (sql: string) => void
}

export const useQueryStore = create<QueryStore>((set) => ({
  sql: '',
  result: null,
  loading: false,
  history: [],
  setSql: (sql) => set({ sql }),
  setResult: (result) => set({ result }),
  setLoading: (loading) => set({ loading }),
  pushHistory: (sql) =>
    set((s) => ({
      history: [sql, ...s.history.filter((h) => h !== sql)].slice(0, 50),
    })),
}))
