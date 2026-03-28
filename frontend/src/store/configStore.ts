import { create } from 'zustand'

import { getConfig } from '@/api/config'

interface ConfigState {
  sourceDir: string
  loaded: boolean
  load: () => Promise<void>
}

export const useConfigStore = create<ConfigState>((set, get) => ({
  sourceDir: '',
  loaded: false,
  load: async () => {
    if (get().loaded) return
    try {
      const res = await getConfig()
      set({ sourceDir: res.data.source_dir ?? '', loaded: true })
    } catch {
      set({ loaded: true })
    }
  },
}))
