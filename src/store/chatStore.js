import { create } from 'zustand';

const initialMessages = [
  { role: 'system', content: 'Neural bridge established. Monitoring session...' },
  { role: 'assistant', content: 'I can help you with terminal commands, debug errors, or suggest solutions. What do you need?' }
];
const cloneInitialMessages = () => initialMessages.map((message) => ({ ...message }));

export const useChatStore = create((set) => ({
  messages: cloneInitialMessages(),
  isLoading: false,
  streamingContent: '',

  setMessages: (messages) => set({ messages }),
  addMessage: (message) => set((state) => ({ 
    messages: [...state.messages, message] 
  })),
  
  setIsLoading: (isLoading) => set({ isLoading }),

  resetChat: () => set({
    messages: cloneInitialMessages(),
    isLoading: false,
    streamingContent: ''
  }),

  updateLastMessage: (content) => set((state) => {
    const newMessages = [...state.messages];
    if (newMessages.length > 0) {
      newMessages[newMessages.length - 1] = { 
        ...newMessages[newMessages.length - 1], 
        content 
      };
    }
    return { messages: newMessages };
  }),

  // Helper for streaming
  appendLastMessage: (chunk) => set((state) => {
    const newMessages = [...state.messages];
    if (newMessages.length > 0) {
      const lastMsg = newMessages[newMessages.length - 1];
      newMessages[newMessages.length - 1] = { 
        ...lastMsg, 
        content: lastMsg.content + chunk 
      };
    }
    return { messages: newMessages };
  }),
}));
