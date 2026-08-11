import { createContext, useContext, useState, useEffect, useCallback } from 'react';

const AuthContext = createContext(null);
const TAB_SESSION_KEY = 'shsh_session_id';

const createSessionId = () => {
  if (window.crypto?.randomUUID) {
    return `tab_${window.crypto.randomUUID().replace(/-/g, '')}`;
  }
  return `tab_${Math.random().toString(36).slice(2)}${Date.now().toString(36)}`;
};

const getTabSessionId = () => {
  let existing = sessionStorage.getItem(TAB_SESSION_KEY);
  if (!existing) {
    existing = createSessionId();
    sessionStorage.setItem(TAB_SESSION_KEY, existing);
  }
  return existing;
};

export const AuthProvider = ({ children }) => {
  const [user, setUser] = useState(null);
  const [loading, setLoading] = useState(true);
  const [sessionId, setSessionId] = useState(getTabSessionId);
  const [sessionReady, setSessionReady] = useState(true);

  const checkAuth = useCallback(async () => {
    try {
      const res = await fetch('/api/me');
      if (res.ok) {
        const data = await res.json();
        setUser(data);
      } else {
        setUser(null);
      }
    } catch (err) {
      console.error('Auth check failed:', err);
      setUser(null);
    } finally {
      setLoading(false);
    }
  }, []);

  const authFetch = useCallback(
    async (url, options = {}) => {
      const headers = new Headers(options.headers || {});
      if (sessionId) {
        headers.set('X-SHSH-Session-ID', sessionId);
      }

      return fetch(url, {
        ...options,
        headers,
      });
    },
    [sessionId]
  );

  const rotateSessionId = useCallback(() => {
    const sid = createSessionId();
    sessionStorage.setItem(TAB_SESSION_KEY, sid);
    setSessionId(sid);
    return sid;
  }, []);

  useEffect(() => {
    checkAuth();
  }, [checkAuth]);

  return (
    <AuthContext.Provider
      value={{
        user,
        loading,
        checkAuth,
        authFetch,
        sessionId,
        sessionReady,
        rotateSessionId,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
};

export const useAuth = () => {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
};
