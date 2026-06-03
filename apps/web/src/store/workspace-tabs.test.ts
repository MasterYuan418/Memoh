import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useChatSelectionStore } from './chat-selection'
import { useWorkspaceTabsStore } from './workspace-tabs'

vi.mock('@/store/chat-list', () => ({
  useChatStore: () => ({
    sessionId: null,
    sessions: [],
    selectSession: vi.fn(),
    createNewSession: vi.fn(),
    isSessionStreaming: vi.fn(() => false),
  }),
}))

describe('workspace-tabs browser tabs', () => {
  beforeEach(() => {
    const storage = new Map<string, string>()
    vi.stubGlobal('localStorage', {
      getItem: (key: string) => storage.get(key) ?? null,
      setItem: (key: string, value: string) => storage.set(key, value),
      removeItem: (key: string) => storage.delete(key),
      clear: () => storage.clear(),
    })
    localStorage.removeItem('workspace-tabs')
    localStorage.removeItem('chat-bot-id')
    localStorage.removeItem('chat-session-id')
    setActivePinia(createPinia())
    useChatSelectionStore().setBot('bot-1')
  })

  it('opens browser tabs and updates their address', () => {
    const store = useWorkspaceTabsStore()

    store.openBrowser()

    expect(store.tabs).toHaveLength(1)
    expect(store.activeTab).toMatchObject({
      id: 'browser:1',
      type: 'browser',
      address: 'localhost:5173/',
    })

    store.updateBrowserAddress('browser:1', 'localhost:3000/app')

    expect(store.activeTab).toMatchObject({
      type: 'browser',
      address: 'localhost:3000/app',
      title: 'localhost:3000/app',
    })
  })
})
