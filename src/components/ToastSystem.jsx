import { useState, useEffect, useCallback } from 'react';
import AlertTriangle from 'lucide-react/dist/esm/icons/alert-triangle';
import Info from 'lucide-react/dist/esm/icons/info';
import X from 'lucide-react/dist/esm/icons/x';
import CheckCircle from 'lucide-react/dist/esm/icons/check-circle';
import XCircle from 'lucide-react/dist/esm/icons/x-circle';

const TOAST_DURATION = 6000;
let toastCounter = 0;

function ToastItem({ toast, onDismiss }) {
    useEffect(() => {
        const timer = setTimeout(() => onDismiss(toast.id), TOAST_DURATION);
        return () => clearTimeout(timer);
    }, [toast.id, onDismiss]);

    const icons = {
        'safety-tier2': <AlertTriangle size={18} className="text-amber-400" />,
        'safety-tier3': <Info size={18} className="text-blue-400" />,
        'error': <XCircle size={18} className="text-red-400" />,
        'success': <CheckCircle size={18} className="text-emerald-400" />
    };

    const bgColors = {
        'safety-tier2': 'bg-amber-500/10 border-amber-500/30',
        'safety-tier3': 'bg-blue-500/10 border-blue-500/30',
        'error': 'bg-red-500/10 border-red-500/30',
        'success': 'bg-emerald-500/10 border-emerald-500/30'
    };

    return (
        <div className={`flex items-start gap-3 p-4 rounded-lg border backdrop-blur-xl shadow-xl animate-in slide-in-from-right duration-300 ${bgColors[toast.type]}`}>
            <div className="flex-none mt-0.5">{icons[toast.type]}</div>
            <div className="flex-1 min-w-0">
                {toast.title && <p className="text-sm font-semibold text-text-primary mb-0.5">{toast.title}</p>}
                <p className="text-sm text-text-secondary">{toast.message}</p>
            </div>
            <button onClick={() => onDismiss(toast.id)} className="flex-none p-1 hover:bg-white/10 rounded transition-colors">
                <X size={14} className="text-text-tertiary" />
            </button>
        </div>
    );
}

export function ToastContainer({ toasts, onDismiss }) {
    if (toasts.length === 0) return null;

    return (
        <div className="fixed top-4 right-4 z-[100] flex flex-col gap-2 max-w-sm w-full pointer-events-none">
            {toasts.map(toast => (
                <div key={toast.id} className="pointer-events-auto">
                    <ToastItem toast={toast} onDismiss={onDismiss} />
                </div>
            ))}
        </div>
    );
}

// Fast refresh rule: hooks must be exported from component modules explicitly.
// eslint-disable-next-line react-refresh/only-export-components
export function useToast() {
    const [toasts, setToasts] = useState([]);

    const addToast = useCallback((toast) => {
        toastCounter += 1;
        const id = `${Date.now()}-${toastCounter}`;
        setToasts(prev => [...prev, { ...toast, id }]);
        return id;
    }, []);

    const dismissToast = useCallback((id) => {
        setToasts(prev => prev.filter(t => t.id !== id));
    }, []);

    const dismissAll = useCallback(() => {
        setToasts([]);
    }, []);

    return { toasts, addToast, dismissToast, dismissAll };
}
