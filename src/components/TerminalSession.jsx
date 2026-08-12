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
import { useAuth } from '../context/useAuth';

// Refined Minimal Dark Theme for xterm.js
const TERMINAL_THEME = {
  background: '#09090b',
  foreground: '#f4f4f5',
  cursor: '#38bdf8',
  cursorAccent: '#09090b',
  selectionBackground: 'rgba(59, 130, 246, 0.35)',
  black: '#18181b',
  red: '#f87171',
  green: '#34d399',
  yellow: '#fbbf24',
  blue: '#60a5fa',
  magenta: '#c084fc',
  cyan: '#38bdf8',
  white: '#f4f4f5',
  brightBlack: '#71717a',
  brightRed: '#fca5a5',
  brightGreen: '#6ee7b7',
  brightYellow: '#fde047',
  brightBlue: '#93c5fd',
  brightMagenta: '#d8b4fe',
  brightCyan: '#7dd3fc',
  brightWhite: '#ffffff',
};

const LeaveTerminalModal = memo(({ onStay, onKeep, onConfirm }) => {
  const stayBtnRef = useRef(null);

  useEffect(() => {
    stayBtnRef.current?.focus();
  }, []);

  return (
    <div
      className="fixed inset-0 z-50 bg-black/70 backdrop-blur-xs flex items-center justify-center p-4"
      onClick={onStay}
    >
      <div
        className="w-full max-w-sm border border-border bg-background-surface p-5 rounded-xl shadow-2xl"
        role="dialog"
        aria-modal="true"
        aria-labelledby="leave-terminal-title"
        onClick={(e) => e.stopPropagation()}
      >
        <h2 id="leave-terminal-title" className="text-sm font-semibold text-text-primary mb-1.5">
          Leave Terminal?
        </h2>
        <p className="text-xs text-text-secondary leading-relaxed mb-5">
          You can keep your session running in the background or destroy the container.
        </p>
        <div className="flex items-center justify-end gap-2 text-xs">
          <button
            ref={stayBtnRef}
            onClick={onStay}
            className="px-3 py-1.5 rounded-md border border-border text-text-secondary hover:text-text-primary hover:bg-zinc-800 transition-colors"
          >
            Stay
          </button>
          <button
            onClick={onKeep}
            className="px-3 py-1.5 rounded-md bg-zinc-800 hover:bg-zinc-700 text-text-primary border border-zinc-700 transition-colors"
          >
            Keep Session
          </button>
          <button
            onClick={onConfirm}
            className="px-3 py-1.5 rounded-md bg-red-500/10 hover:bg-red-500/20 text-red-400 border border-red-500/30 transition-colors"
          >
            Destroy
          </button>
        </div>
      </div>
    </div>
  );
});

// Single Unified Header Bar
const Header = memo(({ status, onDestroy, onToggleChat, isChatOpen, aiEnabled, onHomeClick, onClear, terminalRef }) => {
  const handleCopy = () => {
    const content = terminalRef.current?.getSelection();
    if (content) {
      navigator.clipboard.writeText(content);
    }
  };

  const getStatusIndicator = () => {
    switch (status) {
      case 'connected':
        return (
          <span className="flex items-center gap-1.5 text-xs text-emerald-400 font-medium">
            <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse" />
            <span>Connected</span>
          </span>
        );
      case 'reconnecting':
        return (
          <span className="flex items-center gap-1.5 text-xs text-amber-400 font-medium">
            <span className="w-2 h-2 rounded-full bg-amber-400 animate-pulse" />
            <span>Reconnecting...</span>
          </span>
        );
      default:
        return (
          <span className="flex items-center gap-1.5 text-xs text-red-400 font-medium">
            <span className="w-2 h-2 rounded-full bg-red-400" />
            <span>Disconnected</span>
          </span>
        );
    }
  };

  return (
    <header className="h-12 border-b border-border bg-background-base flex items-center justify-between px-4 shrink-0 z-20 select-none">
      {/* Left: Navigation & Status */}
      <div className="flex items-center gap-4 min-w-0">
        <button
          onClick={onHomeClick}
          className="flex items-center gap-1.5 text-xs font-medium text-text-secondary hover:text-text-primary transition-colors cursor-pointer"
          title="Back to Dashboard"
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <polyline points="15 18 9 12 15 6"></polyline>
          </svg>
          <span>Dashboard</span>
        </button>

        <div className="h-3.5 w-px bg-zinc-800" />

        {getStatusIndicator()}

        <span className="hidden sm:inline-block text-[11px] px-2 py-0.5 rounded bg-zinc-900 border border-zinc-800 text-text-tertiary font-mono">
          Ubuntu 22.04
        </span>
      </div>

      {/* Right: Actions & Tools */}
      <div className="flex items-center gap-2">
        <button
          onClick={handleCopy}
          className="px-2.5 py-1 text-xs text-text-secondary hover:text-text-primary hover:bg-zinc-800/80 rounded transition-colors"
          title="Copy Selection"
        >
          Copy
        </button>

        <button
          onClick={onClear}
          className="px-2.5 py-1 text-xs text-text-secondary hover:text-text-primary hover:bg-zinc-800/80 rounded transition-colors"
          title="Clear Screen"
        >
          Clear
        </button>

        {aiEnabled && (
          <>
            <div className="h-3.5 w-px bg-zinc-800 mx-1" />
            <button
              onClick={onToggleChat}
              className={`flex items-center gap-1.5 px-2.5 py-1 rounded text-xs font-medium transition-colors ${
                isChatOpen
                  ? 'bg-zinc-800 text-zinc-100 border border-zinc-700'
                  : 'text-text-secondary hover:text-text-primary hover:bg-zinc-800/60 border border-transparent'
              }`}
            >
              <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"></path>
              </svg>
              <span>Assistant</span>
            </button>
          </>
        )}

        <div className="h-3.5 w-px bg-zinc-800 mx-1" />

        <button
          onClick={onDestroy}
          className="p-1.5 rounded text-text-tertiary hover:text-red-400 hover:bg-red-500/10 transition-colors"
          title="Terminate Session"
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

// Reconnection Overlay
const ConnectionOverlay = memo(({ onRetry, status }) => {
  const btnRef = useRef(null);
  useEffect(() => btnRef.current?.focus(), []);

  return (
    <div className="absolute inset-0 bg-background-base/90 backdrop-blur-xs z-50 flex items-center justify-center p-4">
      <div className="text-center max-w-sm border border-border bg-background-surface p-6 rounded-xl shadow-2xl">
        {status === 'reconnecting' ? (
          <div className="w-8 h-8 border-2 border-amber-400 border-t-transparent rounded-full animate-spin mx-auto mb-3" />
        ) : (
          <div className="text-red-400 mx-auto mb-3 flex justify-center">
            <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <circle cx="12" cy="12" r="10" />
              <line x1="12" y1="8" x2="12" y2="12" />
              <line x1="12" y1="16" x2="12.01" y2="16" />
            </svg>
          </div>
        )}
        <h3 className="text-text-primary font-semibold mb-1 text-sm">
          {status === 'reconnecting' ? 'Reconnecting to Session' : 'Connection Lost'}
        </h3>
        <p className="text-xs text-text-secondary mb-5 leading-relaxed">
          {status === 'reconnecting'
            ? 'Resuming your environment...'
            : 'Connection to container was interrupted.'}
        </p>
        <button
          ref={btnRef}
          onClick={onRetry}
          className="w-full py-2 bg-zinc-800 hover:bg-zinc-700 text-text-primary text-xs font-medium transition-colors rounded-lg border border-zinc-700"
        >
          {status === 'reconnecting' ? 'Retry Now' : 'Reconnect'}
        </button>
      </div>
    </div>
  );
});

export const TerminalSession = ({ onDestroy }) => {
  const navigate = useNavigate();
  const { sessionId, sessionReady, authFetch, rotateSessionId, checkAuth, clearContainer } = useAuth();
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
  const { toasts, addToast, dismissToast } = useToast();

  useEffect(() => {
    fetch('/api/config')
      .then((res) => res.json())
      .then((data) => {
        if (data.ai_enabled) {
          setAiEnabled(true);
        }
      })
      .catch(() => {
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
    fitAddon.fit();
    sendResize(term.cols, term.rows);
  }, [sendResize]);

  const connect = useCallback(() => {
    if (!mountedRef.current || !sessionReady || !sessionId) return;
    if (socketRef.current) {
      socketRef.current.onclose = null;
      socketRef.current.close();
    }

    const fitAddon = fitAddonRef.current;
    const term = xtermRef.current;
    if (fitAddon && term) fitAddon.fit();
    const cols = term?.cols ?? 80;
    const rows = term?.rows ?? 24;

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsURL =
      `${protocol}//${window.location.host}/ws/terminal` +
      `?session_id=${encodeURIComponent(sessionId)}&cols=${cols}&rows=${rows}`;
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
      socket.send(JSON.stringify({ type: 'resize', cols: term?.cols ?? 80, rows: term?.rows ?? 24 }));
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
          if (
            msg.error === 'container_not_ready' ||
            msg.error === 'failed_to_create_exec' ||
            msg.error === 'user_not_found'
          ) {
            navigate('/provision');
            return;
          }
        } catch {
          /* ignored */
        }
        return;
      }

      if (!initializedRef.current) {
        initTerminalSession();
      }
      xtermRef.current?.write(new Uint8Array(event.data));
    };
  }, [initTerminalSession, navigate, sessionId, sessionReady]);

  const handleTerminate = useCallback(async () => {
    if (terminatingRef.current) return;
    terminatingRef.current = true;

    if (socketRef.current?.readyState === WebSocket.OPEN) {
      socketRef.current.send(JSON.stringify({ type: 'terminate' }));
      socketRef.current.onclose = null;
      socketRef.current.close();
    }

    clearContainer?.();
    resetChat();
    resetChatUI();
    try {
      await authFetch('/api/destroy', { method: 'POST', keepalive: true });
      await checkAuth();
    } catch (err) {
      console.error(err);
    }
    rotateSessionId();
    onDestroy();
  }, [authFetch, checkAuth, clearContainer, onDestroy, resetChat, resetChatUI, rotateSessionId]);

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
      fontSize: 13.5,
      lineHeight: 1.45,
      scrollback: 2500,
      allowProposedApi: true,
      convertEol: true,
    });

    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);
    term.loadAddon(new WebLinksAddon());
    term.loadAddon(new Unicode11Addon());
    term.unicode.activeVersion = '11';
    term.open(terminalRef.current);

    try {
      term.loadAddon(new WebglAddon());
    } catch {
      /* ignore */
    }

    xtermRef.current = term;
    fitAddonRef.current = fitAddon;

    connect();

    const onDataDisposable = term.onData((data) => {
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

  // SSE connection for proactive tips
  useEffect(() => {
    if (!aiEnabled || !sessionReady || !sessionId) return;

    let reconnectTimeout = null;
    let lastEventId = null;
    let reconnectAttempts = 0;
    const maxReconnectAttempts = 10;
    const baseReconnectDelay = 1000;

    const connectEventSource = () => {
      let url = `/api/tutor/stream?session_id=${encodeURIComponent(sessionId)}`;
      if (lastEventId) {
        url += `&lastEventId=${lastEventId}`;
      }

      const eventSource = new EventSource(url, { withCredentials: true });
      eventSourceRef.current = eventSource;

      eventSource.addEventListener('connected', () => {
        reconnectAttempts = 0;
      });

      eventSource.addEventListener('hint', (e) => {
        try {
          const data = JSON.parse(e.data);
          if (data.content) {
            addMessage({
              role: 'assistant',
              content: data.content,
              tools: data.tools_used,
              proactive: true,
            });
            addToast({
              type: 'info',
              title: 'Suggestion',
              message: data.content.length > 80 ? data.content.slice(0, 80) + '...' : data.content,
            });
          }
        } catch {
          /* ignore */
        }
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
    <div className="h-screen bg-background-base flex flex-col overflow-hidden selection:bg-selection selection:text-white">
      <ToastContainer toasts={toasts} onDismiss={dismissToast} />
      <Header
        status={connectionStatus}
        onDestroy={handleTerminate}
        onToggleChat={toggleSidebar}
        isChatOpen={isSidebarOpen}
        aiEnabled={aiEnabled}
        onHomeClick={() => setIsLeaveModalOpen(true)}
        onClear={() => xtermRef.current?.clear()}
        terminalRef={xtermRef}
      />
      <main className="flex-1 flex overflow-hidden">
        <section className="flex-1 flex flex-col bg-background-base relative min-w-0">
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
