// CodeEditor — textarea-based editor with a synced line-number gutter,
// Tab = 4 spaces, Ctrl/Cmd+Enter run shortcut, and a localStorage draft.
// Exposes getValue()/setValue() via ref.

import {
  forwardRef,
  useEffect,
  useImperativeHandle,
  useMemo,
  useRef,
  useState,
} from 'react';

const CodeEditor = forwardRef(function CodeEditor({ storageKey, initial, onRun }, ref) {
  const taRef = useRef(null);
  const gutterRef = useRef(null);
  const [value, setValue] = useState(() => {
    const saved = localStorage.getItem(storageKey);
    return saved !== null ? saved : (initial || '');
  });

  // Persist the draft on every change.
  useEffect(() => {
    localStorage.setItem(storageKey, value);
  }, [storageKey, value]);

  // Keep the gutter aligned after each render (line count may have changed).
  useEffect(() => {
    if (gutterRef.current && taRef.current) gutterRef.current.scrollTop = taRef.current.scrollTop;
  });

  useImperativeHandle(ref, () => ({
    getValue: () => (taRef.current ? taRef.current.value : value),
    setValue: (v) => setValue(v),
  }), [value]);

  const lineNumbers = useMemo(() => {
    const n = value.split('\n').length;
    let out = '';
    for (let i = 1; i <= n; i++) out += `${i}\n`;
    return out;
  }, [value]);

  function onKeyDown(e) {
    if (e.key === 'Tab') {
      e.preventDefault();
      const ta = e.target;
      // setRangeText updates the DOM value and keeps the caret sane;
      // sync React state from it afterwards.
      ta.setRangeText('    ', ta.selectionStart, ta.selectionEnd, 'end');
      setValue(ta.value);
    } else if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
      e.preventDefault();
      if (onRun) onRun();
    }
  }

  function onScroll() {
    if (gutterRef.current && taRef.current) gutterRef.current.scrollTop = taRef.current.scrollTop;
  }

  return (
    <div className="editor-wrap">
      <div className="gutter" ref={gutterRef}>{lineNumbers}</div>
      <textarea
        ref={taRef}
        className="code"
        value={value}
        onChange={(e) => setValue(e.target.value)}
        onKeyDown={onKeyDown}
        onScroll={onScroll}
        spellCheck={false}
        autoComplete="off"
        autoCapitalize="off"
        wrap="off"
        aria-label="Go source code"
      />
    </div>
  );
});

export default CodeEditor;
