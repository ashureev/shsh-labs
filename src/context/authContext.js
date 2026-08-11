import { createContext } from 'react';

export const AuthContext = createContext(null);
export const TAB_SESSION_KEY = 'shsh_session_id';

export const createSessionId = () => {
  if (window.crypto?.randomUUID) {
    return `tab_${window.crypto.randomUUID().replace(/-/g, '')}`;
  }
  return `tab_${Math.random().toString(36).slice(2)}${Date.now().toString(36)}`;
};

export const getTabSessionId = () => {
  let existing = sessionStorage.getItem(TAB_SESSION_KEY);
  if (!existing) {
    existing = createSessionId();
    sessionStorage.setItem(TAB_SESSION_KEY, existing);
  }
  return existing;
};
