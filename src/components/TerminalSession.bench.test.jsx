
/* global global */
import { render, act } from '@testing-library/react';
import { describe, it, vi, expect, beforeEach, afterEach } from 'vitest';
import React, { Profiler } from 'react';
import { MemoryRouter } from 'react-router-dom';
import { TerminalSession } from './TerminalSession';
import * as AuthContext from '../context/AuthContext';
import * as ToastSystem from './ToastSystem';

// Mock dependencies
vi.mock('@xterm/xterm', () => ({
    Terminal: class {
        loadAddon() {}
        open() {}
        dispose() {}
        onData() { return { dispose: () => {} }; }
        writeln() {}
        write() {}
        clear() {}
        unicode = { activeVersion: '11' };
    }
}));
vi.mock('@xterm/addon-fit', () => ({ FitAddon: class { fit() {} } }));
vi.mock('@xterm/addon-web-links', () => ({ WebLinksAddon: class {} }));
vi.mock('@xterm/addon-webgl', () => ({ WebglAddon: class {} }));
vi.mock('@xterm/addon-unicode11', () => ({ Unicode11Addon: class {} }));

// Mock AIChatSidebar to count renders
const aiSidebarRenderSpy = vi.fn();
vi.mock('./AIChatSidebar', () => ({
    AIChatSidebar: (props) => {
        aiSidebarRenderSpy(props);
        return <div data-testid="ai-sidebar">Sidebar</div>;
    }
}));

// Mock ResizeObserver
global.ResizeObserver = class {
    observe() {}
    disconnect() {}
    unobserve() {}
};

// Mock WebSocket
global.WebSocket = class {
    constructor() {}
    close() {}
    send() {}
};

// Mock EventSource
global.EventSource = class {
    constructor() {}
    addEventListener() {}
    close() {}
};

// Mock Element.scrollTo
Element.prototype.scrollTo = () => {};

describe('TerminalSession Performance', () => {
    beforeEach(() => {
        vi.useFakeTimers();
        // Mock AuthContext
        vi.spyOn(AuthContext, 'useAuth').mockReturnValue({
            user: { container_ttl: 3600 },
            checkAuth: vi.fn().mockResolvedValue(true),
            authFetch: vi.fn(),
            sessionId: 'bench-session',
            sessionReady: true,
            rotateSessionId: vi.fn()
        });
        // Mock ToastSystem
        vi.spyOn(ToastSystem, 'useToast').mockReturnValue({
            toasts: [],
            addToast: vi.fn(),
            dismissToast: vi.fn()
        });
        global.fetch = vi.fn().mockResolvedValue({
            json: async () => ({ ai_enabled: true })
        });
    });

    afterEach(() => {
        vi.useRealTimers();
        vi.restoreAllMocks();
        aiSidebarRenderSpy.mockClear();
    });

    it('measures re-renders over time', async () => {
        let renderCount = 0;

        const onRender = () => {
            renderCount++;
        };

        render(
            <MemoryRouter>
                <Profiler id="TerminalSession" onRender={onRender}>
                    <TerminalSession onDestroy={vi.fn()} />
                </Profiler>
            </MemoryRouter>
        );
        await act(async () => {});

        // Initial render
        expect(renderCount).toBeGreaterThan(0);
        const initialCount = renderCount;

        // Advance time by 5 seconds (metrics update interval)
        // Also captures 5 TTL updates (1 per second)
        for (let i = 0; i < 5; i++) {
            act(() => {
                vi.advanceTimersByTime(1000);
            });
        }
        // One more step to hit the 5s metrics interval
        act(() => {
            vi.advanceTimersByTime(100);
        });

        const after5s = renderCount;

        console.log(`Initial renders: ${initialCount}`);
        console.log(`Renders after 5s: ${after5s}`);

        // We expect at least 5 renders from TTL and 1 from Metrics in the current implementation
        // Total should be around initial + 6

        // Advance another 5s
        for (let i = 0; i < 5; i++) {
            act(() => {
                vi.advanceTimersByTime(1000);
            });
        }

        // Assert that AIChatSidebar (expensive component) was not re-rendered unnecessarily.
        // It should render once initially, and once when connection status changes (simulated by WebSocket onopen).
        // It should NOT render for every TTL tick or Metrics tick.
        expect(aiSidebarRenderSpy.mock.calls.length).toBeGreaterThan(0);
        expect(aiSidebarRenderSpy.mock.calls.length).toBeLessThanOrEqual(2);
    });
});
