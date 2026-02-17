import { create } from 'zustand';

export const useChatUIStore = create((set) => ({
  isSidebarOpen: true,

  toggleSidebar: () => set((state) => ({
    isSidebarOpen: !state.isSidebarOpen
  })),

  setSidebarOpen: (isSidebarOpen) => set({ isSidebarOpen }),

  resetChatUI: () => set({ isSidebarOpen: true })
}));
