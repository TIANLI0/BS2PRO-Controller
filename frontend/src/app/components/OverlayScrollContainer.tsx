'use client';

import React, { useCallback, useEffect, useRef, useState } from 'react';

interface OverlayScrollContainerProps {
  children: React.ReactNode;
}

export default function OverlayScrollContainer({ children }: OverlayScrollContainerProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const trackRef = useRef<HTMLDivElement>(null);
  const hideTimerRef = useRef<number | null>(null);
  const dragOffsetRef = useRef(0);

  const [thumbTop, setThumbTop] = useState(0);
  const [thumbHeight, setThumbHeight] = useState(0);
  const [isScrollable, setIsScrollable] = useState(false);
  const [isVisible, setIsVisible] = useState(false);
  const [isDragging, setIsDragging] = useState(false);

  const MIN_THUMB_HEIGHT = 28;
  const AUTO_HIDE_DELAY = 400;

  const clearHideTimer = useCallback(() => {
    if (hideTimerRef.current !== null) {
      window.clearTimeout(hideTimerRef.current);
      hideTimerRef.current = null;
    }
  }, []);

  const scheduleHide = useCallback(() => {
    clearHideTimer();
    hideTimerRef.current = window.setTimeout(() => {
      setIsVisible(false);
    }, AUTO_HIDE_DELAY);
  }, [clearHideTimer]);

  const showScrollbar = useCallback(() => {
    setIsVisible(true);
    if (!isDragging) {
      scheduleHide();
    }
  }, [isDragging, scheduleHide]);

  const updateThumb = useCallback(() => {
    const container = containerRef.current;
    if (!container) return;

    const { scrollTop, scrollHeight, clientHeight } = container;
    const canScroll = scrollHeight > clientHeight + 1;
    setIsScrollable(canScroll);

    if (!canScroll) {
      setThumbTop(0);
      setThumbHeight(0);
      setIsVisible(false);
      return;
    }

    const trackHeight = clientHeight;
    const calculatedThumbHeight = Math.max(
      MIN_THUMB_HEIGHT,
      Math.round((clientHeight / scrollHeight) * trackHeight)
    );
    const maxThumbTop = trackHeight - calculatedThumbHeight;
    const maxScrollTop = scrollHeight - clientHeight;
    const nextThumbTop = maxScrollTop > 0 ? Math.round((scrollTop / maxScrollTop) * maxThumbTop) : 0;

    setThumbHeight(calculatedThumbHeight);
    setThumbTop(nextThumbTop);
  }, []);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    updateThumb();
    showScrollbar();

    const handleScroll = () => {
      updateThumb();
      showScrollbar();
    };

    const handleResize = () => updateThumb();

    container.addEventListener('scroll', handleScroll, { passive: true });
    window.addEventListener('resize', handleResize);

    const resizeObserver = new ResizeObserver(() => updateThumb());
    resizeObserver.observe(container);

    const childObserver = new MutationObserver(() => updateThumb());
    childObserver.observe(container, { childList: true, subtree: true, attributes: true });

    return () => {
      container.removeEventListener('scroll', handleScroll);
      window.removeEventListener('resize', handleResize);
      resizeObserver.disconnect();
      childObserver.disconnect();
      clearHideTimer();
    };
  }, [clearHideTimer, showScrollbar, updateThumb]);

  useEffect(() => {
    if (isDragging) return;
    if (!isScrollable) return;
    scheduleHide();
  }, [isDragging, isScrollable, scheduleHide]);

  const handleThumbPointerDown = useCallback((e: React.PointerEvent<HTMLDivElement>) => {
    const container = containerRef.current;
    const track = trackRef.current;
    if (!container || !track || thumbHeight <= 0) return;

    e.preventDefault();
    const trackRect = track.getBoundingClientRect();
    dragOffsetRef.current = e.clientY - (trackRect.top + thumbTop);

    setIsDragging(true);
    setIsVisible(true);
    clearHideTimer();

    const handlePointerMove = (event: PointerEvent) => {
      const latestTrack = trackRef.current;
      const latestContainer = containerRef.current;
      if (!latestTrack || !latestContainer) return;

      const latestRect = latestTrack.getBoundingClientRect();
      const trackHeight = latestContainer.clientHeight;
      const maxThumbTop = Math.max(0, trackHeight - thumbHeight);
      const rawTop = event.clientY - latestRect.top - dragOffsetRef.current;
      const nextThumbTop = Math.max(0, Math.min(maxThumbTop, rawTop));

      const maxScrollTop = Math.max(0, latestContainer.scrollHeight - latestContainer.clientHeight);
      const nextScrollTop = maxThumbTop > 0 ? (nextThumbTop / maxThumbTop) * maxScrollTop : 0;
      latestContainer.scrollTop = nextScrollTop;
    };

    const handlePointerUp = () => {
      setIsDragging(false);
      scheduleHide();
      window.removeEventListener('pointermove', handlePointerMove);
      window.removeEventListener('pointerup', handlePointerUp);
      window.removeEventListener('pointercancel', handlePointerUp);
    };

    window.addEventListener('pointermove', handlePointerMove);
    window.addEventListener('pointerup', handlePointerUp);
    window.addEventListener('pointercancel', handlePointerUp);
  }, [clearHideTimer, scheduleHide, thumbHeight, thumbTop]);

  return (
    <div className="overlay-scroll-shell">
      <div ref={containerRef} className="app-scroll-root app-scroll-root--hide-native">
        {children}
      </div>

      <div
        ref={trackRef}
        className={`app-overlay-scrollbar ${isVisible && isScrollable ? 'is-visible' : ''}`}
        aria-hidden="true"
      >
        <div
          className="app-overlay-scrollbar-thumb"
          onPointerDown={handleThumbPointerDown}
          style={{
            transform: `translateY(${thumbTop}px)`,
            height: `${thumbHeight}px`,
          }}
        />
      </div>
    </div>
  );
}
