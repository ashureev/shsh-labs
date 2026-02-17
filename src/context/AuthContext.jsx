import { createContext, useContext, useState, useEffect, useCallback, useRef } from 'react';

const AuthContext = createContext(null);
const TAB_SESSION_KEY = 'shsh_session_id';
const SESSION_CHANNEL = 'shsh_session_coord';
const PROBE_TIMEOUT_MS = 120;
const MAX_PROBE_ATTEMPTS = 5;

const createSessionId = () => {
    if (window.crypto?.randomUUID) {
        return `tab_${window.crypto.randomUUID().replace(/-/g, '')}`;
    }
    return `tab_${Math.random().toString(36).slice(2)}${Date.now().toString(36)}`;
};

const getTabSessionId = () => {
    const existing = sessionStorage.getItem(TAB_SESSION_KEY);
    if (existing) return existing;
    const sid = createSessionId();
    sessionStorage.setItem(TAB_SESSION_KEY, sid);
    return sid;
};

export const AuthProvider = ({ children }) => {
    const [user, setUser] = useState(null);
    const [loading, setLoading] = useState(true);
    const [sessionId, setSessionId] = useState('');
    const [sessionReady, setSessionReady] = useState(false);
    const currentSessionRef = useRef('');
    const channelRef = useRef(null);
    const tabIdRef = useRef(createSessionId());
    const pendingRef = useRef(new Map());

    const finalizeSession = useCallback((sid) => {
        currentSessionRef.current = sid;
        sessionStorage.setItem(TAB_SESSION_KEY, sid);
        setSessionId(sid);
        setSessionReady(true);
    }, []);

    const probeCollision = useCallback((sid) => {
        const channel = channelRef.current;
        if (!channel) {
            return Promise.resolve(false);
        }

        const nonce = createSessionId();
        pendingRef.current.set(nonce, { sid, occupied: false });

        channel.postMessage({
            type: 'probe',
            tabId: tabIdRef.current,
            nonce,
            sid
        });

        return new Promise((resolve) => {
            window.setTimeout(() => {
                const state = pendingRef.current.get(nonce);
                pendingRef.current.delete(nonce);
                resolve(Boolean(state?.occupied));
            }, PROBE_TIMEOUT_MS);
        });
    }, []);

    useEffect(() => {
        let cancelled = false;
        const pendingMap = pendingRef.current;

        const resolveSession = async () => {
            const canCoordinate = typeof window !== 'undefined' && typeof BroadcastChannel !== 'undefined';
            if (!canCoordinate) {
                finalizeSession(getTabSessionId());
                return;
            }

            const channel = new BroadcastChannel(SESSION_CHANNEL);
            channelRef.current = channel;

            channel.onmessage = (event) => {
                const msg = event.data;
                if (!msg || typeof msg !== 'object' || msg.tabId === tabIdRef.current) {
                    return;
                }

                if (msg.type === 'probe' && msg.sid && msg.sid === currentSessionRef.current) {
                    channel.postMessage({
                        type: 'occupied',
                        tabId: tabIdRef.current,
                        nonce: msg.nonce,
                        sid: msg.sid
                    });
                    return;
                }

                if (msg.type === 'occupied' && msg.nonce) {
                    const state = pendingMap.get(msg.nonce);
                    if (state && state.sid === msg.sid) {
                        state.occupied = true;
                    }
                }
            };

            let candidate = getTabSessionId();
            for (let i = 0; i < MAX_PROBE_ATTEMPTS; i++) {
                currentSessionRef.current = candidate;
                const occupied = await probeCollision(candidate);
                if (cancelled) {
                    return;
                }
                if (!occupied) {
                    finalizeSession(candidate);
                    return;
                }
                candidate = createSessionId();
            }

            finalizeSession(createSessionId());
        };

        resolveSession();

        return () => {
            cancelled = true;
            pendingMap.clear();
            if (channelRef.current) {
                channelRef.current.close();
                channelRef.current = null;
            }
        };
    }, [finalizeSession, probeCollision]);

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
            console.error("Auth check failed:", err);
            setUser(null);
        } finally {
            setLoading(false);
        }
    }, []);

    // authFetch is a simple wrapper — no auth redirection needed.
    const authFetch = useCallback(async (url, options = {}) => {
        const headers = new Headers(options.headers || {});
        if (sessionId) {
            headers.set('X-SHSH-Session-ID', sessionId);
        }

        return fetch(url, {
            ...options,
            headers
        });
    }, [sessionId]);

    useEffect(() => {
        checkAuth();
    }, [checkAuth]);

    return (
        <AuthContext.Provider value={{ user, loading, checkAuth, authFetch, sessionId, sessionReady }}>
            {children}
        </AuthContext.Provider>
    );
};

// Fast refresh rule: hooks can be exported alongside providers in context modules.
// eslint-disable-next-line react-refresh/only-export-components
export const useAuth = () => {
    const context = useContext(AuthContext);
    if (!context) {
        throw new Error('useAuth must be used within an AuthProvider');
    }
    return context;
};
