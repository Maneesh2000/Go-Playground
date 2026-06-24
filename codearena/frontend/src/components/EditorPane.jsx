// EditorPane — Monaco with a simple open-file tab bar. Ctrl/Cmd+S saves the
// active file via the agent fs service.
import { useEffect } from 'react';
import Editor from '@monaco-editor/react';
import { languageFor } from '../monacoSetup.js';

export default function EditorPane({ files, activePath, onSelect, onClose, onChange, onSave }) {
  const active = files.find((f) => f.path === activePath);

  useEffect(() => {
    const onKey = (e) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 's') {
        e.preventDefault();
        if (activePath) onSave(activePath);
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [activePath, onSave]);

  return (
    <div className="editor-pane">
      <div className="tabbar">
        {files.map((f) => (
          <div
            key={f.path}
            className={`tab ${f.path === activePath ? 'active' : ''} ${f.dirty ? 'dirty' : ''}`}
            onClick={() => onSelect(f.path)}
            title={f.path}
          >
            <span className="tab-name">{f.path.split('/').pop()}{f.dirty ? ' •' : ''}</span>
            <span className="tab-close" onClick={(e) => { e.stopPropagation(); onClose(f.path); }}>×</span>
          </div>
        ))}
      </div>
      <div className="editor-body">
        {active ? (
          <Editor
            key={active.path}
            path={active.path}
            language={languageFor(active.path)}
            value={active.content}
            theme="vs-dark"
            onChange={(v) => onChange(active.path, v ?? '')}
            options={{
              fontSize: 13,
              minimap: { enabled: false },
              scrollBeyondLastLine: false,
              automaticLayout: true,
              tabSize: 2,
            }}
          />
        ) : (
          <div className="editor-empty">Select a file from the tree to start editing.</div>
        )}
      </div>
    </div>
  );
}
