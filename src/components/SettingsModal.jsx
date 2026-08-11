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
        setStatus('Settings updated successfully!');
        setTimeout(() => {
          setStatus('');
          onClose();
        }, 1200);
      } else {
        setStatus('Failed to save settings');
      }
    } catch {
      setStatus('Network error saving settings');
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm p-4 font-mono">
      <div className="bg-[#12141a] border border-[#2e3440] rounded-xl w-full max-w-lg overflow-hidden shadow-2xl">
        <div className="flex justify-between items-center px-6 py-4 border-b border-[#2e3440] bg-[#1a1d26]">
          <div className="flex items-center gap-2">
            <span className="text-indigo-400 font-bold">⚙ LLM & AI Mentor Settings</span>
          </div>
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-white text-lg font-bold"
          >
            ✕
          </button>
        </div>

        <form onSubmit={handleSave} className="p-6 space-y-4 text-xs text-gray-300">
          <div>
            <label className="block text-gray-400 uppercase tracking-wider mb-2 font-bold">
              Provider
            </label>
            <div className="grid grid-cols-2 gap-2">
              {[
                { id: 'ollama', label: 'Ollama (Local Offline)' },
                { id: 'gemini', label: 'Google Gemini' },
                { id: 'openai', label: 'OpenAI' },
                { id: 'openrouter', label: 'OpenRouter' },
              ].map((p) => (
                <button
                  type="button"
                  key={p.id}
                  onClick={() => handleProviderChange(p.id)}
                  className={`py-2 px-3 text-left rounded border transition-all ${
                    provider === p.id
                      ? 'border-indigo-500 bg-indigo-950/40 text-indigo-300 font-bold'
                      : 'border-[#2e3440] bg-[#1a1d26] text-gray-400 hover:border-gray-500'
                  }`}
                >
                  {p.label}
                </button>
              ))}
            </div>
          </div>

          <div>
            <label className="block text-gray-400 uppercase tracking-wider mb-1 font-bold">
              Model Name
            </label>
            <input
              type="text"
              value={model}
              onChange={(e) => setModel(e.target.value)}
              placeholder="e.g. gemini-2.5-flash, llama3.2, gpt-4o-mini"
              className="w-full bg-[#1a1d26] border border-[#2e3440] rounded px-3 py-2 text-white focus:outline-none focus:border-indigo-500"
            />
          </div>

          {provider !== 'ollama' && (
            <div>
              <label className="block text-gray-400 uppercase tracking-wider mb-1 font-bold">
                API Key {hasKey && <span className="text-emerald-400 text-[10px] lowercase">(configured: {maskedKey})</span>}
              </label>
              <input
                type="password"
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
                placeholder={hasKey ? 'Enter new key to replace' : 'Paste your API key here'}
                className="w-full bg-[#1a1d26] border border-[#2e3440] rounded px-3 py-2 text-white focus:outline-none focus:border-indigo-500"
              />
            </div>
          )}

          <div>
            <label className="block text-gray-400 uppercase tracking-wider mb-1 font-bold">
              Base URL Endpoint
            </label>
            <input
              type="text"
              value={baseURL}
              onChange={(e) => setBaseURL(e.target.value)}
              placeholder="http://localhost:11434/v1"
              className="w-full bg-[#1a1d26] border border-[#2e3440] rounded px-3 py-2 text-white focus:outline-none focus:border-indigo-500 text-[11px]"
            />
          </div>

          {status && (
            <div className="p-2 rounded bg-indigo-950/60 border border-indigo-500/50 text-indigo-300 text-center font-bold">
              {status}
            </div>
          )}

          <div className="flex justify-end gap-3 pt-4 border-t border-[#2e3440]">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 rounded bg-[#1a1d26] hover:bg-[#252a36] text-gray-300 transition-colors"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={isSaving}
              className="px-5 py-2 rounded bg-indigo-600 hover:bg-indigo-500 text-white font-bold transition-colors disabled:opacity-50"
            >
              {isSaving ? 'Saving...' : 'Save & Apply'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
