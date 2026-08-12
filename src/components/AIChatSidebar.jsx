import { useState, useRef, useEffect, useCallback, memo } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { useAuth } from '../context/useAuth';
import { useChatStore } from '../store/chatStore';
import { useChatUIStore } from '../store/chatUIStore';
import { SettingsModal } from './SettingsModal';

// Clean Code Block with Copy & Run in Terminal Action
const CodeBlock = ({ language, code, onInsert }) => {
  const [copied, setCopied] = useState(false);
  const isShell = !language || ['bash', 'sh', 'shell', 'zsh'].includes(language.toLowerCase());

  const handleCopy = () => {
    navigator.clipboard.writeText(code);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="my-2.5 rounded-lg border border-zinc-800 overflow-hidden bg-zinc-950">
      <div className="flex justify-between items-center px-3 py-1.5 bg-zinc-900 border-b border-zinc-800">
        <span className="text-[10px] text-zinc-400 font-mono uppercase">
          {language || 'shell'}
        </span>
        <div className="flex items-center gap-2">
          {isShell && onInsert && (
            <button
              onClick={() => onInsert(code)}
              className="text-[10px] text-sky-400 hover:text-sky-300 font-medium transition-colors cursor-pointer"
              title="Insert and run command in active terminal"
            >
              ↳ Run in Terminal
            </button>
          )}
          <button
            onClick={handleCopy}
            className="text-[10px] text-zinc-400 hover:text-zinc-200 transition-colors cursor-pointer"
          >
            {copied ? 'Copied' : 'Copy'}
          </button>
        </div>
      </div>
      <pre className="p-3 m-0 overflow-x-auto text-xs text-zinc-200 font-mono">
        <code>{code}</code>
      </pre>
    </div>
  );
};

// Message Item Component
const Message = memo(({ message, onInsert }) => {
  const isBot = message.role === 'assistant';
  const isSystem = message.role === 'system';

  if (isSystem) {
    return (
      <div className="text-zinc-500 text-xs italic py-1 px-2 border-l-2 border-zinc-800">
        {message.content}
      </div>
    );
  }

  if (isBot) {
    return (
      <div className="flex flex-col gap-1.5 mb-3">
        <div className="flex items-center gap-1.5 text-[11px] font-medium text-zinc-400">
          <span className="w-1.5 h-1.5 rounded-full bg-sky-400" />
          <span>Assistant</span>
        </div>

        <div className="p-3 rounded-lg bg-zinc-900/70 border border-zinc-800/80 text-zinc-200 text-xs leading-relaxed">
          {message.tools && message.tools.length > 0 && (
            <div className="flex flex-wrap gap-1 mb-2">
              {message.tools.map((tool, idx) => (
                <span
                  key={idx}
                  className="text-[10px] px-2 py-0.5 rounded bg-zinc-800/90 border border-zinc-700 text-zinc-400 font-mono"
                >
                  Ran: {tool}
                </span>
              ))}
            </div>
          )}
          <div className="markdown-body">
            <ReactMarkdown
              remarkPlugins={[remarkGfm]}
              components={{
                code({ inline, className, children, ...props }) {
                  const match = /language-(\w+)/.exec(className || '');
                  return !inline ? (
                    <CodeBlock
                      language={match ? match[1] : ''}
                      code={String(children).replace(/\n$/, '')}
                      onInsert={onInsert}
                    />
                  ) : (
                    <code {...props}>
                      {children}
                    </code>
                  );
                },
              }}
            >
              {message.content}
            </ReactMarkdown>
          </div>
        </div>
      </div>
    );
  }

  // User Message
  return (
    <div className="flex flex-col items-end mb-3">
      <div className="max-w-[88%] p-3 rounded-lg bg-zinc-800/90 border border-zinc-700/80 text-zinc-100 text-xs leading-relaxed">
        {message.content}
      </div>
    </div>
  );
});

export const AIChatSidebar = memo(({ onInsertCommand }) => {
  const messages = useChatStore((state) => state.messages);
  const addMessage = useChatStore((state) => state.addMessage);
  const setIsLoading = useChatStore((state) => state.setIsLoading);
  const isLoading = useChatStore((state) => state.isLoading);
  const isSidebarOpen = useChatUIStore((state) => state.isSidebarOpen);
  const toggleSidebar = useChatUIStore((state) => state.toggleSidebar);
  const { authFetch } = useAuth();

  const [input, setInput] = useState('');
  const [isSettingsOpen, setIsSettingsOpen] = useState(false);
  const scrollRef = useRef(null);
  const [width, setWidth] = useState(380);
  const [isResizing, setIsResizing] = useState(false);

  // Connect to live SSE stream for ambient hints
  useEffect(() => {
    const eventSource = new EventSource('/api/tutor/stream');

    eventSource.addEventListener('hint', (e) => {
      try {
        const hint = JSON.parse(e.data);
        if (hint.content) {
          addMessage({
            role: 'assistant',
            content: hint.content,
            tools: hint.tools_used,
            proactive: true,
          });
        }
      } catch {
        // Ignore malformed SSE messages
      }
    });

    return () => {
      eventSource.close();
    };
  }, [addMessage]);

  const startResizing = useCallback((e) => {
    setIsResizing(true);
    e.preventDefault();
  }, []);

  const stopResizing = useCallback(() => {
    setIsResizing(false);
  }, []);

  const resize = useCallback(
    (e) => {
      if (isResizing) {
        const newWidth = window.innerWidth - e.clientX;
        if (newWidth >= 300 && newWidth <= 750) {
          setWidth(newWidth);
        }
      }
    },
    [isResizing]
  );

  useEffect(() => {
    if (isResizing) {
      window.addEventListener('mousemove', resize);
      window.addEventListener('mouseup', stopResizing);
      document.body.style.cursor = 'ew-resize';
      document.body.style.userSelect = 'none';
    } else {
      window.removeEventListener('mousemove', resize);
      window.removeEventListener('mouseup', stopResizing);
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
    }
    return () => {
      window.removeEventListener('mousemove', resize);
      window.removeEventListener('mouseup', stopResizing);
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
    };
  }, [isResizing, resize, stopResizing]);

  useEffect(() => {
    if (scrollRef.current && messages.length > 0) {
      scrollRef.current.scrollTo({
        top: scrollRef.current.scrollHeight,
        behavior: 'smooth',
      });
    }
  }, [messages, isLoading]);

  const handleSubmit = async (e) => {
    e.preventDefault();
    if (!input.trim() || isLoading) return;

    const userMsg = { role: 'user', content: input.trim() };
    addMessage(userMsg);
    setInput('');
    setIsLoading(true);

    try {
      const resp = await authFetch('/api/chat', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ message: userMsg.content }),
      });

      if (!resp.ok) throw new Error('Failed to reach assistant');

      const data = await resp.json();
      addMessage({
        role: 'assistant',
        content: data.answer || 'No response',
        tools: data.tools_used || [],
      });
    } catch {
      addMessage({
        role: 'assistant',
        content: 'Unable to process the request. Please check your AI settings or try again.',
      });
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <>
      <aside
        style={{ width: isSidebarOpen ? `${width}px` : '0px' }}
        className={`flex-none bg-background-surface border-l border-border flex flex-col relative z-10 transition-all duration-150 ${
          isSidebarOpen ? 'block' : 'hidden'
        }`}
      >
        {isSidebarOpen && (
          <>
            {/* Resize Handle */}
            <div
              onMouseDown={startResizing}
              className={`absolute left-0 top-0 bottom-0 w-1.5 cursor-ew-resize z-30 transition-colors hover:bg-zinc-600 ${
                isResizing ? 'bg-zinc-500' : ''
              }`}
            />

            {/* Header */}
            <div className="h-12 border-b border-border flex items-center justify-between px-4 bg-background-base select-none">
              <div className="flex items-center gap-2">
                <span className="text-text-primary font-semibold text-xs">AI Assistant</span>
              </div>
              <div className="flex items-center gap-2">
                <button
                  onClick={() => setIsSettingsOpen(true)}
                  className="p-1 rounded text-text-secondary hover:text-text-primary hover:bg-zinc-800 transition-colors"
                  title="Settings"
                >
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                    <circle cx="12" cy="12" r="3"></circle>
                    <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"></path>
                  </svg>
                </button>
                <button
                  onClick={toggleSidebar}
                  className="p-1 rounded text-text-secondary hover:text-text-primary hover:bg-zinc-800 transition-colors"
                  title="Close Assistant"
                >
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                    <line x1="18" y1="6" x2="6" y2="18"></line>
                    <line x1="6" y1="6" x2="18" y2="18"></line>
                  </svg>
                </button>
              </div>
            </div>

            {/* Messages */}
            <div
              ref={scrollRef}
              className="flex-1 overflow-y-auto p-4 custom-scrollbar"
            >
              {messages.map((m, i) => (
                <Message key={i} message={m} onInsert={onInsertCommand} />
              ))}
              {isLoading && (
                <div className="flex items-center gap-2 text-zinc-400 text-xs py-2">
                  <div className="w-3 h-3 border border-zinc-400 border-t-transparent rounded-full animate-spin" />
                  <span>Thinking...</span>
                </div>
              )}
            </div>

            {/* Input Form */}
            <div className="p-3 border-t border-border bg-background-base z-20">
              <form onSubmit={handleSubmit}>
                <div className="relative flex items-center">
                  <input
                    type="text"
                    value={input}
                    onChange={(e) => setInput(e.target.value)}
                    placeholder="Ask a question or request command help..."
                    disabled={isLoading}
                    className="w-full bg-zinc-900 border border-zinc-800 focus:border-zinc-600 rounded-lg py-2 pl-3 pr-9 text-xs text-text-primary placeholder-zinc-500 outline-none transition-colors disabled:opacity-50"
                  />
                  <button
                    type="submit"
                    disabled={isLoading || !input.trim()}
                    className="absolute right-2 p-1 text-zinc-400 hover:text-zinc-100 disabled:opacity-30 cursor-pointer"
                    title="Send"
                  >
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                      <line x1="22" y1="2" x2="11" y2="13"></line>
                      <polygon points="22 2 15 22 11 13 2 9 22 2"></polygon>
                    </svg>
                  </button>
                </div>
              </form>
            </div>
          </>
        )}
      </aside>

      <SettingsModal
        isOpen={isSettingsOpen}
        onClose={() => setIsSettingsOpen(false)}
      />
    </>
  );
});
