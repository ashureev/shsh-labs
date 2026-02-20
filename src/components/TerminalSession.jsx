import { useEffect, useRef, useState, useCallback, memo } from 'react';
import { useNavigate } from 'react-router-dom';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { WebLinksAddon } from '@xterm/addon-web-links';
import { WebglAddon } from '@xterm/addon-webgl';
import { Unicode11Addon } from '@xterm/addon-unicode11';
import '@xterm/xterm/css/xterm.css';
import { AIChatSidebar } from './AIChatSidebar';
import { ToastContainer, useToast } from './ToastSystem';
import { useChatStore } from '../store/chatStore';
import { useChatUIStore } from '../store/chatUIStore';
import { useAuth } from '../context/AuthContext';

// Tokyo Night Terminal Theme
const TERMINAL_THEME = {
    background: '#0b0b0b',
    foreground: '#c0caf5',
    cursor: '#bb9af7',
    cursorAccent: '#ffffff',
    selectionBackground: '#283457',
    black: '#15161e',
    red: '#f7768e',
    green: '#9ece6a',
    yellow: '#e0af68',
    blue: '#7aa2f7',
    magenta: '#bb9af7',
    cyan: '#7dcfff',
    white: '#a9b1d6',
    brightBlack: '#414868',
    brightRed: '#ff9e64',
    brightGreen: '#73daca',
    brightYellow: '#e0af68',
    brightBlue: '#7aa2f7',
    brightMagenta: '#bb9af7',
    brightCyan: '#7dcfff',
    brightWhite: '#c0caf5',
};

// Toggle Switch Component
const ToggleSwitch = ({ checked, onChange }) => (
    <label className="toggle-switch">
        <input type="checkbox" checked={checked} onChange={onChange} />
        <span className="toggle-slider"></span>
    </label>
);

const LeaveTerminalModal = memo(({ onStay, onKeep, onConfirm }) => {
    const stayBtnRef = useRef(null);

    useEffect(() => {
        stayBtnRef.current?.focus();
    }, []);

    return (
        <div className="absolute inset-0 z-50 bg-bg/80 flex items-center justify-center p-4" onClick={onStay}>
            <div
                className="w-full max-w-md border border-border bg-panel p-5 rounded-sm"
                role="dialog"
                aria-modal="true"
                aria-labelledby="leave-terminal-title"
                onClick={(e) => e.stopPropagation()}
            >
                <h2 id="leave-terminal-title" className="text-sm font-medium text-fg mb-2">Leave terminal session?</h2>
                <p className="text-xs text-muted leading-relaxed mb-5">
                    Leaving from this breadcrumb will destroy the active terminal session.
                </p>
                <div className="flex items-center justify-end gap-2">
                    <button
                        ref={stayBtnRef}
                        onClick={onStay}
                        className="px-3 py-1.5 border border-border text-xs text-muted hover:text-fg transition-colors"
                    >
                        Stay
                    </button>
                    <button
                        onClick={onKeep}
                        className="px-3 py-1.5 border border-border text-xs text-fg hover:bg-border/40 transition-colors"
                    >
                        Leave & keep session
                    </button>
                    <button
                        onClick={onConfirm}
                        className="px-3 py-1.5 border border-term-red text-xs text-term-red hover:bg-term-red/10 transition-colors"
                    >
                        Leave & destroy
                    </button>
                </div>
            </div>
        </div>
    );
});

// Header Component - Tokyo Night TUI Style
const Header = memo(({ status, onDestroy, onToggleChat, isChatOpen, sessionInfo, aiEnabled, onHomeClick }) => {

    const getStatusDot = () => {
        switch (status) {
            case 'connected':
                return 'bg-term-green shadow-[0_0_8px_rgba(158,206,106,0.4)]';
            case 'reconnecting':
                return 'bg-term-yellow animate-pulse';
            default:
                return 'bg-term-red';
        }
    };

    return (
        <header className="h-10 tui-border-b bg-bg flex items-center justify-between px-4 shrink-0 z-20 select-none">
            {/* Left: Session Info */}
            <div className="flex items-center gap-4 min-w-0">
                <div className="flex items-center gap-1 text-[11px] font-medium text-muted uppercase tracking-wider">
                    <button
                        onClick={onHomeClick}
                        className="hover:text-fg transition-colors"
                        aria-label="Go to home"
                    >
                        Home
                    </button>
                    <span className="text-border">/</span>
                    <span className="text-fg">Terminal</span>
                </div>
                <div className="flex items-center gap-2 text-xs font-medium text-muted min-w-0">
                    <span className={`w-2 h-2 rounded-full ${getStatusDot()}`}></span>
                    <span className="text-fg truncate">{sessionInfo?.name || 'shsh-session'}</span>
                    <span className="text-muted">/</span>
                    <span>{sessionInfo?.node || 'node-01'}</span>
                </div>
            </div>

            {/* Right: Controls */}
            <div className="flex items-center gap-6 text-xs font-medium text-muted">
                {aiEnabled && (
                    <>
                        <div className="h-3 w-px bg-border"></div>

                        {/* AI Toggle Switch in Status Bar */}
                        <div className="flex items-center gap-2">
                            <span className="text-[10px] font-bold text-muted uppercase tracking-wider">Agent</span>
                            <ToggleSwitch checked={isChatOpen} onChange={onToggleChat} />
                        </div>
                    </>
                )}

                <div className="h-3 w-px bg-border"></div>

                {/* Power/Destroy Button */}
                <button
                    onClick={onDestroy}
                    className="flex items-center gap-2 hover:text-term-red cursor-pointer transition-colors"
                    title="Destroy Session"
                >
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                        <path d="M18.36 6.64a9 9 0 1 1-12.73 0"></path>
                        <line x1="12" y1="2" x2="12" y2="12"></line>
                    </svg>
                </button>
            </div>
        </header>
    );
});

// Connection Overlay
const ConnectionOverlay = memo(({ onRetry, status }) => {
    const btnRef = useRef(null);
    useEffect(() => btnRef.current?.focus(), []);

    return (
        <div className="absolute inset-0 bg-bg/95 z-50 flex items-center justify-center p-4">
            <div className="text-center max-w-sm border border-border bg-panel p-6 rounded-sm">
                {status === 'reconnecting' ? (
                    <div className="w-8 h-8 border-2 border-term-yellow border-t-transparent rounded-full animate-spin mx-auto mb-4"></div>
                ) : (
                    <div className="text-term-red mx-auto mb-4">
                        <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                            <circle cx="12" cy="12" r="10"></circle>
                            <line x1="12" y1="8" x2="12" y2="12"></line>
                            <line x1="12" y1="16" x2="12.01" y2="16"></line>
                        </svg>
                    </div>
                )}
                <h3 className="text-fg font-medium mb-2 text-sm">
                    {status === 'reconnecting' ? 'Connection Unstable' : 'Connection Lost'}
                </h3>
                <p className="text-xs text-muted mb-6 leading-relaxed">
                    {status === 'reconnecting'
                        ? 'Attempting to resume your environment. Your data is safe.'
                        : 'We lost communication with your playground. Please check your network.'}
                </p>
                <button
                    ref={btnRef}
                    onClick={onRetry}
                    className="w-full py-2 bg-border/50 text-fg text-xs font-medium hover:bg-border transition-colors border border-border"
                >
                    {status === 'reconnecting' ? 'Retry Immediately' : 'Try Reconnecting'}
                </button>
            </div>
        </div>
    );
});

// Terminal Toolbar
const TerminalToolbar = memo(({ onClear, terminalRef }) => {
    const handleCopy = () => {
        const content = terminalRef.current?.getSelection();
        if (content) navigator.clipboard.writeText(content);
    };

    return (
        <div className="flex items-center px-4 py-2 tui-border-b bg-bg">
            <div className="flex items-center gap-2">
                <span className="text-xs text-muted uppercase tracking-wider">Terminal</span>
            </div>
            <div className="flex items-center gap-4 ml-auto">
                <button
                    onClick={onClear}
                    className="text-xs text-muted hover:text-fg transition-colors"
                >
                    Clear
                </button>
                <button
                    onClick={handleCopy}
                    className="text-xs text-muted hover:text-fg transition-colors"
                >
                    Copy
                </button>
            </div>
        </div>
    );
});

export const TerminalSession = ({ onDestroy }) => {
    const navigate = useNavigate();
    const { sessionId, sessionReady, authFetch, rotateSessionId } = useAuth();
    const addMessage = useChatStore((state) => state.addMessage);
    const resetChat = useChatStore((state) => state.resetChat);
    const isSidebarOpen = useChatUIStore((state) => state.isSidebarOpen);
    const toggleSidebar = useChatUIStore((state) => state.toggleSidebar);
    const resetChatUI = useChatUIStore((state) => state.resetChatUI);
    const terminalRef = useRef(null);
    const xtermRef = useRef(null);
    const fitAddonRef = useRef(null);
    const socketRef = useRef(null);
    const initializedRef = useRef(false);
    const reconnectAttemptsRef = useRef(0);
    const resizeTimeoutRef = useRef(null);
    const heartbeatIntervalRef = useRef(null);
    const eventSourceRef = useRef(null);
    const mountedRef = useRef(true);
    const terminatingRef = useRef(false);

    const [connectionStatus, setConnectionStatus] = useState('connecting');
    const [aiEnabled, setAiEnabled] = useState(false);
    const [isLeaveModalOpen, setIsLeaveModalOpen] = useState(false);
    const [sessionInfo] = useState({
        name: 'shsh-session',
        node: 'node-01',
    });
    const { toasts, addToast, dismissToast } = useToast();

    // Fetch config to check if AI is enabled
    useEffect(() => {
        fetch('/api/config')
            .then(res => res.json())
            .then(data => {
                if (data.ai_enabled) {
                    setAiEnabled(true);
                }
            })
            .catch(() => {
                // AI disabled by default on error
                setAiEnabled(false);
            });
    }, []);

    const sendResize = useCallback((cols, rows) => {
        if (!mountedRef.current) return;
        if (socketRef.current?.readyState === WebSocket.OPEN) {
            if (resizeTimeoutRef.current) clearTimeout(resizeTimeoutRef.current);
            resizeTimeoutRef.current = setTimeout(() => {
                if (socketRef.current?.readyState === WebSocket.OPEN) {
                    socketRef.current.send(JSON.stringify({ type: 'resize', cols, rows }));
                }
            }, 60);
        }
    }, []);

    const initTerminalSession = useCallback(() => {
        if (!mountedRef.current || initializedRef.current || !xtermRef.current) return;
        initializedRef.current = true;
        const term = xtermRef.current;
        const fitAddon = fitAddonRef.current;
        term.writeln('\x1b[38;2;187;154;247m[SYSTEM] AUTHENTICATED AND CONNECTED\x1b[0m\r\n');
        fitAddon.fit();
        sendResize(term.cols, term.rows);
    }, [sendResize]);

    const connect = useCallback(() => {
        if (!mountedRef.current || !sessionReady || !sessionId) return;
        if (socketRef.current) {
            socketRef.current.onclose = null;
            socketRef.current.close();
        }

        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsURL = `${protocol}//${window.location.host}/ws/terminal?session_id=${encodeURIComponent(sessionId)}`;
        const socket = new WebSocket(wsURL);
        socket.binaryType = 'arraybuffer';
        socketRef.current = socket;

        socket.onopen = () => {
            if (!mountedRef.current) {
                socket.close(1000);
                return;
            }
            setConnectionStatus('connected');
            reconnectAttemptsRef.current = 0;
            heartbeatIntervalRef.current = setInterval(() => {
                if (socket.readyState === WebSocket.OPEN && mountedRef.current) {
                    socket.send(JSON.stringify({ type: 'ping' }));
                }
            }, 20000);
        };

        socket.onclose = (event) => {
            clearInterval(heartbeatIntervalRef.current);
            if (!mountedRef.current || event.code === 1000) {
                setConnectionStatus('disconnected');
                return;
            }

            setConnectionStatus('reconnecting');
            const delay = Math.min(Math.pow(2, reconnectAttemptsRef.current) * 1000, 10000);
            reconnectAttemptsRef.current++;
            setTimeout(() => {
                if (mountedRef.current) connect();
            }, delay);
        };

        socket.onerror = () => setConnectionStatus('disconnected');

        socket.onmessage = (event) => {
            if (typeof event.data === 'string') {
                try {
                    const msg = JSON.parse(event.data);
                    if (msg.type === 'pong') return;
                } catch { /* ignored */ }
                return;
            }

            if (!initializedRef.current) {
                initTerminalSession();
            }
            xtermRef.current?.write(new Uint8Array(event.data));
        };
    }, [initTerminalSession, sessionId, sessionReady]);

    const handleTerminate = useCallback(async () => {
        if (terminatingRef.current) return;
        terminatingRef.current = true;

        if (socketRef.current?.readyState === WebSocket.OPEN) {
            socketRef.current.send(JSON.stringify({ type: 'terminate' }));
            socketRef.current.onclose = null;
            socketRef.current.close();
        }

        resetChat();
        resetChatUI();
        try {
            await authFetch('/api/destroy', { method: 'POST', keepalive: true });
        } catch (err) {
            console.error(err);
        }
        rotateSessionId();
        onDestroy();
    }, [authFetch, onDestroy, resetChat, resetChatUI, rotateSessionId]);

    useEffect(() => {
        if (!isLeaveModalOpen) return;
        const onKeyDown = (event) => {
            if (event.key === 'Escape') setIsLeaveModalOpen(false);
        };
        window.addEventListener('keydown', onKeyDown);
        return () => window.removeEventListener('keydown', onKeyDown);
    }, [isLeaveModalOpen]);

    useEffect(() => {
        mountedRef.current = true;
        if (!terminalRef.current) return;
        const term = new Terminal({
            cursorBlink: true,
            theme: TERMINAL_THEME,
            fontFamily: '"JetBrains Mono", monospace',
            fontSize: 14,
            lineHeight: 1.5,
            scrollback: 2000,
            allowProposedApi: true,
            convertEol: true,
        });

        const fitAddon = new FitAddon();
        term.loadAddon(fitAddon);
        term.loadAddon(new WebLinksAddon());
        term.loadAddon(new Unicode11Addon());
        term.unicode.activeVersion = '11';
        term.open(terminalRef.current);

        try { term.loadAddon(new WebglAddon()); } catch { /* ignore */ }

        xtermRef.current = term;
        fitAddonRef.current = fitAddon;

        connect();

        const onDataDisposable = term.onData(data => {
            if (socketRef.current?.readyState === WebSocket.OPEN) {
                socketRef.current.send(JSON.stringify({ type: 'data', content: data }));
            }
        });

        const resizeObserver = new ResizeObserver(() => {
            fitAddon.fit();
            sendResize(term.cols, term.rows);
        });
        resizeObserver.observe(terminalRef.current);

        return () => {
            mountedRef.current = false;
            resizeObserver.disconnect();
            onDataDisposable.dispose();
            if (socketRef.current) {
                socketRef.current.onclose = null;
                socketRef.current.close(1000);
            }
            clearInterval(heartbeatIntervalRef.current);
            term.dispose();
            xtermRef.current = null;
            resetChat();
            resetChatUI();
        };
    }, [connect, resetChat, resetChatUI, sendResize]);

    // SSE connection for safety warnings and proactive tips (only if AI enabled)
    useEffect(() => {
        if (!aiEnabled || !sessionReady || !sessionId) return;

        let reconnectTimeout = null;
        let lastEventId = null;
        let reconnectAttempts = 0;
        const maxReconnectAttempts = 10;
        const baseReconnectDelay = 1000;

        const connectEventSource = () => {
            let url = `/api/agent/stream?session_id=${encodeURIComponent(sessionId)}`;
            if (lastEventId) {
                url += `&lastEventId=${lastEventId}`;
            }

            const eventSource = new EventSource(url, { withCredentials: true });
            eventSourceRef.current = eventSource;

            eventSource.addEventListener('connected', (e) => {
                reconnectAttempts = 0;
                try {
                    const data = JSON.parse(e.data);
                    if (data.event_id) lastEventId = data.event_id;
                } catch { /* ignore */ }
            });

            eventSource.addEventListener('message', (e) => {
                if (e.lastEventId) lastEventId = e.lastEventId;

                try {
                    const data = JSON.parse(e.data);
                    if (useChatUIStore.getState().isSidebarOpen && (data.content || data.sidebar)) {
                        addMessage({
                            role: 'assistant',
                            content: data.sidebar || data.content,
                            type: data.type,
                            proactive: true
                        });
                    }
                    if (data.type === 'safety-tier2' || data.type === 'safety-tier3') {
                        addToast({
                            type: data.type,
                            title: data.type === 'safety-tier2' ? 'Confirm Intent' : 'Security Notice',
                            message: data.sidebar || data.content || 'Command detected'
                        });
                    }
                } catch { /* ignore */ }
            });

            eventSource.addEventListener('error', () => {
                if (eventSource.readyState === EventSource.CLOSED && reconnectAttempts < maxReconnectAttempts) {
                    reconnectAttempts++;
                    const delay = Math.min(baseReconnectDelay * Math.pow(2, reconnectAttempts - 1), 30000);
                    reconnectTimeout = setTimeout(connectEventSource, delay);
                }
            });
        };

        connectEventSource();

        return () => {
            if (reconnectTimeout) clearTimeout(reconnectTimeout);
            if (eventSourceRef.current) {
                eventSourceRef.current.close();
                eventSourceRef.current = null;
            }
        };
    }, [addMessage, addToast, aiEnabled, sessionId, sessionReady]);

    return (
        <div className="h-screen bg-bg flex flex-col overflow-hidden selection:bg-selection selection:text-white">
            <ToastContainer toasts={toasts} onDismiss={dismissToast} />
            <Header
                status={connectionStatus}
                onDestroy={handleTerminate}
                onToggleChat={toggleSidebar}
                isChatOpen={isSidebarOpen}
                sessionInfo={sessionInfo}
                aiEnabled={aiEnabled}
                onHomeClick={() => setIsLeaveModalOpen(true)}
            />
            <main className="flex-1 flex overflow-hidden">
                <section className="flex-1 flex flex-col bg-bg relative min-w-0">
                    <TerminalToolbar
                        onClear={() => xtermRef.current?.clear()}
                        terminalRef={xtermRef}
                    />
                    <div className="flex-1 relative p-2">
                        <div ref={terminalRef} className="absolute inset-2" />
                        {(connectionStatus === 'disconnected' || connectionStatus === 'reconnecting') && (
                            <ConnectionOverlay status={connectionStatus} onRetry={connect} />
                        )}
                        {isLeaveModalOpen && (
                            <LeaveTerminalModal
                                onStay={() => setIsLeaveModalOpen(false)}
                                onKeep={() => navigate('/')}
                                onConfirm={handleTerminate}
                            />
                        )}
                    </div>
                </section>
                {aiEnabled && <AIChatSidebar />}
            </main>
        </div>
    );
};
