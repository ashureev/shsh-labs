import { useState, useRef, useEffect, useCallback, memo } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { useAuth } from '../context/AuthContext';
import { useChatStore } from '../store/chatStore';
import { useChatUIStore } from '../store/chatUIStore';

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
                return 'AGT';
            case 'user':
            case 'usr':
                return 'USR';
            default:
                return 'SYS';
        }
    };

    return (
        <span className={`role-badge ${getBadgeClass()}`}>
            {getLabel()}
        </span>
    );
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
        <div className="my-3">
            <div className="flex justify-between items-center px-3 py-1.5 bg-term-black border border-border border-b-0">
                <span className="text-[10px] text-muted font-mono uppercase">{language || 'bash'}</span>
                <button
                    onClick={handleCopy}
                    className="text-[10px] text-muted hover:text-fg transition-colors"
                >
                    {copied ? 'Copied' : 'Copy'}
                </button>
            </div>
            <pre className="bg-term-black p-3 m-0 overflow-x-auto border border-border">
                <code className="text-xs text-term-cyan font-mono">{code}</code>
            </pre>
        </div>
    );
};

// Message Component
const Message = memo(({ message, isLatest }) => {
    const isBot = message.role === 'assistant';
    const isProactive = message.proactive;
    const isSystem = message.role === 'system';

    if (isSystem || message.role === 'sys') {
        return (
            <div className={`flex gap-3 mb-4 ${isLatest ? 'latest-message' : ''}`}>
                <div className="w-8 shrink-0 text-center">
                    <RoleBadge role="sys" />
                </div>
                <div className="text-muted text-xs italic border-l border-border pl-3 py-0.5">
                    {message.content}
                </div>
            </div>
        );
    }

    if (isBot) {
        return (
            <div className={`flex gap-3 mb-4 ${isLatest ? 'latest-message' : ''}`}>
                <div className="w-8 shrink-0 text-center pt-0.5">
                    <RoleBadge role="agt" />
                </div>
                <div className="flex-1 text-fg leading-relaxed text-sm">
                    {isProactive && (
                        <div className="mb-2">
                            <span className="text-[10px] text-muted uppercase tracking-wider border border-border px-1.5 py-0.5 rounded">
                                Suggestion
                            </span>
                        </div>
                    )}
                    <div className="prose prose-invert prose-sm max-w-none">
                        <ReactMarkdown
                            remarkPlugins={[remarkGfm]}
                            components={{
                                code({ inline, className, children, ...props }) {
                                    const match = /language-(\w+)/.exec(className || '');
                                    const codeStr = String(children).replace(/\n$/, '');

                                    if (!inline && match) {
                                        return <CodeBlock language={match[1]} code={codeStr} />;
                                    }
                                    return (
                                        <code
                                            className="bg-term-black px-1.5 py-0.5 rounded text-term-cyan text-xs font-mono"
                                            {...props}
                                        >
                                            {children}
                                        </code>
                                    );
                                },
                                p: ({ children }) => <p className="mb-2 last:mb-0 leading-relaxed text-fg/80">{children}</p>,
                                h1: ({ children }) => <h1 className="text-sm font-semibold mb-2 text-fg">{children}</h1>,
                                h2: ({ children }) => <h2 className="text-xs font-semibold mb-2 text-fg">{children}</h2>,
                                h3: ({ children }) => <h3 className="text-xs font-medium mb-2 text-fg">{children}</h3>,
                                ul: ({ children }) => <ul className="list-disc pl-4 mb-2 space-y-1 text-fg/80">{children}</ul>,
                                ol: ({ children }) => <ol className="list-decimal pl-4 mb-2 space-y-1 text-fg/80">{children}</ol>,
                                li: ({ children }) => <li className="pl-1 text-fg/80">{children}</li>,
                                blockquote: ({ children }) => (
                                    <blockquote className="border-l border-border pl-3 py-1 my-2 text-muted italic">
                                        {children}
                                    </blockquote>
                                ),
                                strong: ({ children }) => <strong className="text-term-red font-medium">{children}</strong>,
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
            <div className="w-8 shrink-0 text-center pt-0.5">
                <RoleBadge role="usr" />
            </div>
            <div className="flex-1 text-fg leading-relaxed text-sm">
                {message.content}
            </div>
        </div>
    );
});

export const AIChatSidebar = memo(() => {
    const { authFetch } = useAuth();
    const messages = useChatStore((state) => state.messages);
    const addMessage = useChatStore((state) => state.addMessage);
    const setIsLoading = useChatStore((state) => state.setIsLoading);
    const isLoading = useChatStore((state) => state.isLoading);
    const updateLastMessage = useChatStore((state) => state.updateLastMessage);
    const isSidebarOpen = useChatUIStore((state) => state.isSidebarOpen);
    
    const [input, setInput] = useState('');
    const scrollRef = useRef(null);

    // Throttling refs for streaming updates
    const streamBufferRef = useRef('');
    const flushIntervalRef = useRef(null);
    const pendingContentRef = useRef('');
    const isStreamingRef = useRef(false);

    const [width, setWidth] = useState(400);
    const [isResizing, setIsResizing] = useState(false);

    const startResizing = useCallback((e) => {
        setIsResizing(true);
        e.preventDefault();
    }, []);

    const stopResizing = useCallback(() => {
        setIsResizing(false);
    }, []);

    const resize = useCallback((e) => {
        if (isResizing) {
            const newWidth = window.innerWidth - e.clientX;
            if (newWidth >= 300 && newWidth <= 700) {
                setWidth(newWidth);
            }
        }
    }, [isResizing]);

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
                behavior: 'smooth'
            });
        }
    }, [messages, isLoading]);

    // Flush accumulated stream content to state
    const flushStreamBuffer = useCallback(() => {
        if (!isStreamingRef.current) {
            flushIntervalRef.current = null;
            return;
        }

        if (pendingContentRef.current !== streamBufferRef.current) {
            pendingContentRef.current = streamBufferRef.current;
            updateLastMessage(streamBufferRef.current);
        }
        flushIntervalRef.current = requestAnimationFrame(flushStreamBuffer);
    }, [updateLastMessage]);

    // Cleanup flush interval on unmount
    useEffect(() => {
        return () => {
            isStreamingRef.current = false;
            if (flushIntervalRef.current) {
                cancelAnimationFrame(flushIntervalRef.current);
                flushIntervalRef.current = null;
            }
        };
    }, []);

    const handleSubmit = async (e) => {
        e.preventDefault();
        if (!input.trim() || isLoading) return;

        const userMsg = { role: 'user', content: input.trim() };
        addMessage(userMsg);
        setInput('');
        setIsLoading(true);

        const botMsg = { role: 'assistant', content: '' };
        addMessage(botMsg);

        // Reset stream buffer
        streamBufferRef.current = '';
        pendingContentRef.current = '';
        isStreamingRef.current = true;

        // Start throttled flush interval (using rAF)
        flushIntervalRef.current = requestAnimationFrame(flushStreamBuffer);

        try {
            const resp = await authFetch('/api/agent/chat', {
                method: 'POST',
                body: JSON.stringify({ message: userMsg.content })
            });

            if (!resp.ok) throw new Error('Failed');

            const reader = resp.body.getReader();
            const decoder = new TextDecoder();
            let buffer = '';

            while (true) {
                const { done, value } = await reader.read();
                if (value) buffer += decoder.decode(value, { stream: true });
                const lines = buffer.split('\n');
                if (done) buffer = '';
                else buffer = lines.pop() || '';

                for (const line of lines) {
                    const trimmed = line.trim();
                    if (!trimmed || !trimmed.startsWith('data:')) continue;
                    try {
                        const data = JSON.parse(trimmed.slice(5));
                        if (data.response) {
                            // Accumulate in ref instead of updating state immediately
                            streamBufferRef.current += data.response;
                        }
                    } catch { /* ignore */ } 
                }
                if (done) break;
            }
        } catch {
            streamBufferRef.current = 'Sorry, something went wrong. Please try again.';
        } finally {
            // Clear flush interval and do final update
            isStreamingRef.current = false;
            if (flushIntervalRef.current) {
                cancelAnimationFrame(flushIntervalRef.current);
                flushIntervalRef.current = null;
            }
            // Final flush to ensure all content is displayed
            updateLastMessage(streamBufferRef.current);
            setIsLoading(false);
        }
    };

    return (
        <aside
            style={{ width: isSidebarOpen ? `${width}px` : '0px' }}
            className={`flex-none bg-bg border-term-cyan/40 flex flex-col relative z-10 transition-all duration-200 ${isSidebarOpen ? 'border-l-2 sidebar-glow' : 'border-l-0'}`}
        >
            {isSidebarOpen && (
                <>
                    {/* Resize Handle */}
                    <div
                        onMouseDown={startResizing}
                        className={`absolute left-0 top-0 bottom-0 w-1 cursor-ew-resize z-30 transition-colors hover:bg-term-cyan/30 ${isResizing ? 'bg-term-cyan/50' : ''}`}
                    />

                    {/* Header */}
                    <div className="h-9 tui-border-b flex items-center px-4 bg-bg select-none">
                        <div className="flex items-center gap-2">
                            <span className="text-term-magenta font-bold text-sm">AI_COPILOT</span>
                            <span className="text-muted text-xs">[ACTIVE]</span>
                        </div>
                    </div>

                    {/* Messages */}
                    <div
                        ref={scrollRef}
                        className="flex-1 overflow-y-auto p-4 space-y-2 custom-scrollbar"
                    >
                        {messages.map((m, i) => <Message key={i} message={m} isLatest={i === messages.length - 1} />)}
                        {isLoading && (
                            <div className="flex gap-3">
                                <div className="w-8 shrink-0 text-center pt-0.5">
                                    <RoleBadge role="agt" />
                                </div>
                                <div className="flex items-center gap-2 text-muted">
                                    <span className="text-sm animate-pulse">Thinking</span>
                                    <span className="text-sm">...</span>
                                </div>
                            </div>
                        )}
                    </div>

                    {/* Input */}
                    <div className="p-3 tui-border-t bg-bg z-20">
                        <form onSubmit={handleSubmit}>
                            <div className="relative input-prompt">
                                <input
                                    type="text"
                                    value={input}
                                    onChange={(e) => setInput(e.target.value)}
                                    placeholder="Ask Copilot..."
                                    disabled={isLoading}
                                    className="w-full bg-bg border-b border-border focus:border-term-magenta focus:ring-0 rounded-none py-2 pl-6 pr-8 text-sm font-mono text-fg placeholder-muted outline-none transition-colors disabled:opacity-50"
                                />
                                <div className="absolute right-2 top-1/2 -translate-y-1/2 text-[10px] font-bold text-muted opacity-50">
                                    RET
                                </div>
                            </div>
                        </form>
                    </div>
                </>
            )}
        </aside>
    );
});
