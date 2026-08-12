import { useState, useEffect } from 'react';

export function SettingsModal({ isOpen, onClose }) {
  const [provider, setProvider] = useState('ollama');
  const [model, setModel] = useState('llama3.2');
  const [apiKey, setApiKey] = useState('');
  const [baseURL, setBaseURL] = useState('');
  const [maskedKey, setMaskedKey] = useState('');
  const [hasKey, setHasKey] = useState(false);
  const [status, setStatus] = useState('');
  const [isSaving, setIsSaving] = useState(false);

  useEffect(() => {
    if (isOpen) {
      fetch('/api/settings')
        .then((res) => res.json())
        .then((data) => {
          setProvider(data.provider || 'ollama');
          setModel(data.model || 'llama3.2');
          setBaseURL(data.base_url || '');
          setMaskedKey(data.masked_api_key || '');
          setHasKey(data.has_api_key || false);
        })
        .catch(() => {});
    }
  }, [isOpen]);

  if (!isOpen) return null;

  const handleProviderChange = (newProvider) => {
    setProvider(newProvider);
    if (newProvider === 'gemini') {
      setModel('gemini-2.5-flash');
      setBaseURL('https://generativelanguage.googleapis.com');
    } else if (newProvider === 'openai') {
      setModel('gpt-4o-mini');
      setBaseURL('https://api.openai.com/v1');
    } else if (newProvider === 'openrouter') {
      setModel('meta-llama/llama-3.3-70b-instruct');
      setBaseURL('https://openrouter.ai/api/v1');
    } else if (newProvider === 'ollama') {
      setModel('llama3.2');
      setBaseURL('http://localhost:11434/v1');
    }
  };

  const handleSave = async (e) => {
    e.preventDefault();
    setIsSaving(true);
    setStatus('');
    try {
      const res = await fetch('/api/settings', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          provider,
          model,
          api_key: apiKey,
          base_url: baseURL,
        }),
      });
      if (res.ok) {
        setStatus('Settings saved successfully');
        setTimeout(() => {
          setStatus('');
          onClose();
        }, 1000);
      } else {
        setStatus('Failed to save settings');
      }
    } catch {
      setStatus('Error connecting to server');
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-xs p-4"
      onClick={onClose}
    >
      <div
        className="bg-background-surface border border-border rounded-xl w-full max-w-md overflow-hidden shadow-2xl"
        role="dialog"
        aria-modal="true"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex justify-between items-center px-5 py-4 border-b border-border bg-background-base">
          <div>
            <h2 className="text-sm font-semibold text-text-primary">AI Settings</h2>
            <p className="text-xs text-text-tertiary mt-0.5">Configure model provider & credentials</p>
          </div>
          <button
            onClick={onClose}
            className="p-1 rounded text-text-secondary hover:text-text-primary hover:bg-zinc-800 transition-colors"
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <line x1="18" y1="6" x2="6" y2="18" />
              <line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          </button>
        </div>

        <form onSubmit={handleSave} className="p-5 space-y-4 text-xs">
          <div>
            <label className="block text-text-secondary font-medium mb-1.5">
              Provider
            </label>
            <div className="grid grid-cols-2 gap-2">
              {[
                { id: 'ollama', label: 'Ollama (Local)' },
                { id: 'gemini', label: 'Google Gemini' },
                { id: 'openai', label: 'OpenAI' },
                { id: 'openrouter', label: 'OpenRouter' },
              ].map((p) => (
                <button
                  type="button"
                  key={p.id}
                  onClick={() => handleProviderChange(p.id)}
                  className={`py-2 px-3 text-left rounded-lg border text-xs transition-all ${
                    provider === p.id
                      ? 'border-zinc-600 bg-zinc-800 text-zinc-100 font-medium'
                      : 'border-zinc-800 bg-zinc-900/60 text-zinc-400 hover:text-zinc-200 hover:border-zinc-700'
                  }`}
                >
                  {p.label}
                </button>
              ))}
            </div>
          </div>

          <div>
            <label className="block text-text-secondary font-medium mb-1">
              Model Name
            </label>
            <input
              type="text"
              value={model}
              onChange={(e) => setModel(e.target.value)}
              placeholder="e.g. gemini-2.5-flash, llama3.2, gpt-4o-mini"
              className="w-full bg-zinc-900 border border-zinc-800 rounded-lg px-3 py-2 text-text-primary font-mono text-xs focus:outline-none focus:border-zinc-600"
            />
          </div>

          {provider !== 'ollama' && (
            <div>
              <div className="flex justify-between items-center mb-1">
                <label className="text-text-secondary font-medium">
                  API Key
                </label>
                {hasKey && (
                  <span className="text-[11px] text-emerald-400">
                    Active: {maskedKey}
                  </span>
                )}
              </div>
              <input
                type="password"
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
                placeholder={hasKey ? 'Enter new key to replace' : 'Paste API key'}
                className="w-full bg-zinc-900 border border-zinc-800 rounded-lg px-3 py-2 text-text-primary text-xs focus:outline-none focus:border-zinc-600"
              />
            </div>
          )}

          <div>
            <label className="block text-text-secondary font-medium mb-1">
              Base URL Endpoint
            </label>
            <input
              type="text"
              value={baseURL}
              onChange={(e) => setBaseURL(e.target.value)}
              placeholder="http://localhost:11434/v1"
              className="w-full bg-zinc-900 border border-zinc-800 rounded-lg px-3 py-2 text-text-primary font-mono text-xs focus:outline-none focus:border-zinc-600"
            />
          </div>

          {status && (
            <div className="p-2 rounded-lg bg-zinc-800 border border-zinc-700 text-zinc-200 text-center text-xs">
              {status}
            </div>
          )}

          <div className="flex justify-end gap-2 pt-3 border-t border-border">
            <button
              type="button"
              onClick={onClose}
              className="px-3.5 py-1.5 rounded-lg border border-border bg-zinc-900 hover:bg-zinc-800 text-text-secondary hover:text-text-primary transition-colors text-xs"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={isSaving}
              className="px-4 py-1.5 rounded-lg bg-zinc-100 hover:bg-white text-zinc-900 font-medium text-xs transition-colors disabled:opacity-50 cursor-pointer"
            >
              {isSaving ? 'Saving...' : 'Save Settings'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
