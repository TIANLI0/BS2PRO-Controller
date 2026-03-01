import { useEffect } from 'react';
import { useAppStore } from '../store/app-store';

export function useAppBootstrap() {
  const initializeApp = useAppStore((state) => state.initializeApp);
  const startEventListeners = useAppStore((state) => state.startEventListeners);

  useEffect(() => {
    initializeApp();
  }, [initializeApp]);

  useEffect(() => {
    const stopListening = startEventListeners();
    return () => {
      stopListening();
    };
  }, [startEventListeners]);
}
