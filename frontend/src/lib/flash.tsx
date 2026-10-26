import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { useLocation } from 'react-router-dom';

export interface FlashMessages {
    success?: string | null;
    error?: string | null;
    message?: string | null;
}

interface FlashStore {
    flash: FlashMessages;
    setFlash: (flash: FlashMessages) => void;
}

const FlashContext = createContext<FlashStore | null>(null);

/**
 * One-shot feedback for actions whose result has no place of its own on the
 * screen. The store clears on navigation, which is the boundary the server
 * session flash used: a message belongs to the screen that produced it.
 */
export function FlashProvider({ children }: { children: ReactNode }) {
    const [flash, setFlash] = useState<FlashMessages>({});
    const location = useLocation();

    useEffect(() => {
        setFlash({});
    }, [location.pathname]);

    const value = useMemo(() => ({ flash, setFlash }), [flash]);

    return <FlashContext.Provider value={value}>{children}</FlashContext.Provider>;
}

export function useFlash(): FlashStore {
    const store = useContext(FlashContext);
    if (!store) throw new Error('useFlash must be used inside FlashProvider');
    return store;
}
