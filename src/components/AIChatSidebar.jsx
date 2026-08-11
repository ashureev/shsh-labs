import { useState, useRef, useEffect, useCallback, memo } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { useAuth } from '../context/useAuth';
import { useChatStore } from '../store/chatStore';
import { useChatUIStore } from '../store/chatUIStore';
import { SettingsModal } from './SettingsModal';

// Role Badge Component
const RoleBadge = ({ role }) => {
  const getBadgeClass = () => {
    switch (role) {
      case 'system':
      case 'sys':
        return 'role-badge-sys';
      case 'assistant':
      case 'agt':
        return 'role-badge-agt';
      case 'user':
      case 'usr':
        return 'role-badge-usr';
      default:
        return 'role-badge-sys';
    }
  };

  const getLabel = () => {
    switch (role) {
      case 'system':
      case 'sys':
        return 'SYS';
      case 'assistant':
      case 'agt':
        return 'MENTOR';
      case 'user':
      case 'usr':
        return 'YOU';
      default:
        return 'SYS';
    }
  };

  return <span className={`role-badge ${getBadgeClass()}`}>{getLabel()}</span>;
};

// Code Block with Copy
const CodeBlock = ({ language, code }) => {
  const [copied, setCopied] = useState(false);

  const handleCopy = () => {
    navigator.clipboard.writeText(code);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="my-2 rounded border border-[#2e3440] overflow-hidden bg-[#12141a]">
      <div className="flex justify-between items-center px-3 py-1 bg-[#1a1d26] border-b border-[#2e3440]">
        <span className="text-[10px] text-gray-400 font-mono uppercase">{language || 'bash'}</span>
        <button
          onClick={handleCopy}
          className="text-[10px] text-indigo-400 hover:text-indigo-300 transition-colors"
        >
          {copied ? 'Copied' : 'Copy'}
        </button>
      </div>
      <pre className="p-3 m-0 overflow-x-auto text-xs text-emerald-400 font-mono">
        <code>{code}</code>
      </pre>
    </div>
  );
};

// Message Component
const Message = memo(({ message, isLatest }) => {
  const isBot = message.role === 'assistant';
  const isSystem = message.role === 'system';

  if (isSystem) {
    return (
      <div className={`flex gap-3 mb-3 ${isLatest ? 'latest-message' : ''}`}>
        <div className="w-8 shrink-0 text-center">
          <RoleBadge role="sys" />
        </div>
        <div className="text-gray-400 text-xs italic border-l border-[#2e3440] pl-3 py-0.5">
          {message.content}
        </div>
      </div>
    );
  }

  if (isBot) {
    return (
      <div className={`flex gap-3 mb-4 ${isLatest ? 'latest-message' : ''}`}>
        <div className="w-14 shrink-0 text-center pt-0.5">
          <RoleBadge role="assistant" />
        </div>
        <div className="flex-1 min-w-0">
          {message.tools && message.tools.length > 0 && (
            <div className="flex flex-wrap gap-1 mb-2">
              {message.tools.map((t, idx) => (
                <span
                  key={idx}
                  className="text-[10px] px-2 py-0.5 rounded bg-indigo-950/80 border border-indigo-500/40 text-indigo-300 font-mono"
                >
                  🔍 {t}
                </span>
              ))}
            </div>
          )}
          <div className="p-3 rounded-lg bg-[#1a1d26] border border-[#2e3440] text-gray-200 text-xs leading-relaxed">
            <ReactMarkdown
              remarkPlugins={[remarkGfm]}
              components={{
                code({ inline, className, children, ...props }) {
                  const match = /language-(\w+)/.exec(className || '');
                  return !inline && match ? (
                    <CodeBlock
                      language={match[1]}
                      code={String(children).replace(/\n$/, '')}
                    />
                  ) : (
                    <code className="bg-[#12141a] px-1 py-0.5 rounded text-indigo-300 font-mono text-[11px]" {...props}>
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

  return (
    <div className={`flex gap-3 mb-4 ${isLatest ? 'latest-message' : ''}`}>
      <div className="w-14 shrink-0 text-center pt-0.5">
        <RoleBadge role="user" />
      </div>
      <div className="flex-1 min-w-0 p-3 rounded-lg bg-indigo-950/40 border border-indigo-500/30 text-indigo-100 text-xs leading-relaxed">
        {message.content}
      </div>
    </div>
  );
});

export const AIChatSidebar = memo(() => {
  const messages = useChatStore((state) => state.messages);
  const addMessage = useChatStore((state) => state.addMessage);
  const setIsLoading = useChatStore((state) => state.setIsLoading);
  const isLoading = useChatStore((state) => state.isLoading);
  const isSidebarOpen = useChatUIStore((state) => state.isSidebarOpen);
  const { authFetch } = useAuth();

  const [input, setInput] = useState('');
  const [isSettingsOpen, setIsSettingsOpen] = useState(false);
  const scrollRef = useRef(null);
  const [width, setWidth] = useState(420);
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
        if (newWidth >= 320 && newWidth <= 800) {
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

      if (!resp.ok) throw new Error('Failed to reach mentor');

      const data = await resp.json();
      addMessage({
        role: 'assistant',
        content: data.answer || 'No response',
        tools: data.tools_used || [],
      });
    } catch {
      addMessage({
        role: 'assistant',
        content: 'Sorry, I had trouble analyzing the request. Please check the AI settings or try again.',
      });
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <>
      <aside
        style={{ width: isSidebarOpen ? `${width}px` : '0px' }}
        className={`flex-none bg-[#0e1015] border-l border-[#2e3440] flex flex-col relative z-10 transition-all duration-200 ${
          isSidebarOpen ? 'block' : 'hidden'
        }`}
      >
        {isSidebarOpen && (
          <>
            {/* Resize Handle */}
            <div
              onMouseDown={startResizing}
              className={`absolute left-0 top-0 bottom-0 w-1.5 cursor-ew-resize z-30 transition-colors hover:bg-indigo-500/50 ${
                isResizing ? 'bg-indigo-500' : ''
              }`}
            />

            {/* Header */}
            <div className="h-12 border-b border-[#2e3440] flex items-center justify-between px-4 bg-[#141720] select-none">
              <div className="flex items-center gap-2">
                <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse"></span>
                <span className="text-white font-bold text-xs tracking-wider">AI AMBIENT MENTOR</span>
              </div>
              <button
                onClick={() => setIsSettingsOpen(true)}
                className="text-xs px-2.5 py-1 rounded border border-[#2e3440] hover:border-indigo-400 text-gray-300 hover:text-white transition-colors bg-[#1a1d26]"
              >
                ⚙ Settings
              </button>
            </div>

            {/* Messages */}
            <div
              ref={scrollRef}
              className="flex-1 overflow-y-auto p-4 space-y-2 custom-scrollbar font-mono"
            >
              {messages.map((m, i) => (
                <Message key={i} message={m} isLatest={i === messages.length - 1} />
              ))}
              {isLoading && (
                <div className="flex gap-3">
                  <div className="w-14 shrink-0 text-center pt-0.5">
                    <RoleBadge role="assistant" />
                  </div>
                  <div className="flex items-center gap-2 text-indigo-400 text-xs py-2">
                    <span className="animate-pulse">Inspecting sandbox & reasoning...</span>
                  </div>
                </div>
              )}
            </div>

            {/* Input */}
            <div className="p-3 border-t border-[#2e3440] bg-[#141720] z-20">
              <form onSubmit={handleSubmit}>
                <div className="relative">
                  <input
                    type="text"
                    value={input}
                    onChange={(e) => setInput(e.target.value)}
                    placeholder="Ask mentor a question or explain an error..."
                    disabled={isLoading}
                    className="w-full bg-[#1a1d26] border border-[#2e3440] focus:border-indigo-500 rounded-lg py-2.5 pl-3 pr-10 text-xs font-mono text-white placeholder-gray-500 outline-none transition-colors disabled:opacity-50"
                  />
                  <button
                    type="submit"
                    disabled={isLoading || !input.trim()}
                    className="absolute right-2 top-1/2 -translate-y-1/2 text-xs font-bold text-indigo-400 hover:text-indigo-300 disabled:opacity-30"
                  >
                    ↵
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
