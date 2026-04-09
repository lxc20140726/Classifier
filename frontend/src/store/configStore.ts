import { create } from 'zustand'

import { getConfig } from '@/api/config'
import type { ConfiguredPathOption } from '@/types'

interface ConfigState {
  sourceDir: string
  scanInputDirs: string[]
  pathOptions: ConfiguredPathOption[]
  loaded: boolean
  load: (force?: boolean) => Promise<void>
}

export const useConfigStore = create<ConfigState>((set, get) => ({
  sourceDir: '',
  scanInputDirs: [],
  pathOptions: [],
  loaded: false,
  load: async (force = false) => {
    if (get().loaded && !force) return
    try {
      const res = await getConfig()
      set({
        sourceDir: res.data.source_dir ?? '',
        scanInputDirs: res.data.scan_input_dirs ?? [],
        pathOptions: res.data.path_options ?? [],
        loaded: true,
      })
    } catch {
      set({ loaded: true })
    }
  },
}))
