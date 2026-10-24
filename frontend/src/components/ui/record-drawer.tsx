"use client";
import { useEffect, useId, useRef, type ReactNode } from "react";
export function RecordDrawer({
  title,
  onClose,
  children,
  footer,
  centered = false,
  wide = false,
  extraWide = false,
  returnFocusTo,
}: {
  title: string;
  onClose: () => void;
  children: ReactNode;
  footer?: ReactNode;
  centered?: boolean;
  wide?: boolean;
  extraWide?: boolean;
  returnFocusTo?: HTMLElement | null;
}) {
  const ref = useRef<HTMLDialogElement>(null);
  const titleId = useId();
  useEffect(() => {
    const dialog = ref.current;
    const opener = returnFocusTo ?? document.activeElement as HTMLElement | null;
    const overflow = document.body.style.overflow;
    dialog?.showModal();
    document.body.style.overflow = "hidden";
    return () => {
      dialog?.close();
      document.body.style.overflow = overflow;
      opener?.focus();
    };
  }, [returnFocusTo]);
  return (
    <dialog
      ref={ref}
      aria-labelledby={titleId}
      onCancel={(e) => {
        e.preventDefault();
        e.stopPropagation();
        onClose();
      }}
      className={`vtpl-modal fixed ${centered ? `inset-0 m-auto h-fit max-h-[90dvh] w-[calc(100%-2rem)] ${extraWide ? "max-w-[1600px]" : wide ? "max-w-6xl" : "max-w-lg"} rounded-xl` : `inset-y-0 left-auto right-0 m-0 h-dvh max-h-none w-full ${extraWide ? "max-w-[1600px]" : wide ? "max-w-6xl" : "max-w-4xl"}`} border-0 bg-slate-50 p-0 text-slate-900 shadow-2xl backdrop:bg-slate-950/45 ${footer ? "open:flex flex-col overflow-hidden" : "overflow-y-auto"}`}
    >
      <div className="sticky top-0 z-10 flex shrink-0 items-center justify-between gap-3 border-b border-slate-200 bg-white px-5 py-4">
        <h2 id={titleId} className="text-lg font-bold">
          {title}
        </h2>
        <button
          type="button"
          aria-label="Close panel"
          className="min-h-11 min-w-11 rounded-lg border border-slate-200 text-xl"
          onClick={onClose}
        >
          ×
        </button>
      </div>
      <div className={`space-y-4 p-4 sm:p-6 ${footer ? "min-h-0 flex-1 overflow-y-auto" : ""}`}>{children}</div>
      {footer ? <div className="shrink-0 border-t border-slate-200 bg-white p-4 sm:px-6">{footer}</div> : null}
    </dialog>
  );
}
