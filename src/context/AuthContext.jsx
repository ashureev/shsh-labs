import { useState, useEffect, useCallback } from 'react';
import { AuthContext, createSessionId, getTabSessionId, TAB_SESSION_KEY } from './authContext';

export const AuthProvider = ({ children }) => {
  const [user, setUser] = useState(null);
  const [loading, setLoading] = useState(true);
  const [sessionId, setSessionId] = useState(getTabSessionId);
  const [sessionReady] = useState(true);

  const checkAuth = useCallback(async () => {
    try {
      const res = await fetch(`/api/me?_t=${Date.now()}`, {
        cache: 'no-store',
        headers: {
          'Cache-Control': 'no-cache',
        },
      });
      if (res.ok) {
        const data = await res.json();
        setUser(data);
        return data;
      } else {
        setUser(null);
        return null;
      }
    } catch (err) {
      console.error('Auth check failed:', err);
      setUser(null);
      return null;
    } finally {
      setLoading(false);
    }
  }, []);

  const clearContainer = useCallback(() => {
    setUser((prev) => (prev ? { ...prev, container_id: '' } : null));
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
        clearContainer,
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
