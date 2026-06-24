// FileTree — react-arborist tree fed by the agent fs service. Loads the tree
// eagerly (skipping heavy dirs) which is fine for the small workspaces this MVP
// targets; lazy-loading on expand is a later enhancement.
import { useEffect, useState, useCallback } from 'react';
import { Tree } from 'react-arborist';

const SKIP_DIRS = new Set(['node_modules', '.git', 'dist', '.cache', 'venv', '__pycache__']);

async function buildTree(conn, path, depth, budget) {
  const entries = await conn.list(path || '/');
  const nodes = [];
  for (const e of entries) {
    if (budget.n++ > 1000) break;
    const full = path ? `${path}/${e.name}` : e.name;
    if (e.is_dir) {
      let children = [];
      if (!SKIP_DIRS.has(e.name) && depth < 10) {
        children = await buildTree(conn, full, depth + 1, budget);
      }
      nodes.push({ id: full, name: e.name, isDir: true, children });
    } else {
      nodes.push({ id: full, name: e.name, isDir: false });
    }
  }
  return nodes;
}

export default function FileTree({ conn, refreshKey, onOpenFile }) {
  const [data, setData] = useState([]);
  const [error, setError] = useState(null);

  const reload = useCallback(() => {
    if (!conn) return;
    buildTree(conn, '', 0, { n: 0 })
      .then((d) => { setData(d); setError(null); })
      .catch((e) => setError(e.message));
  }, [conn]);

  useEffect(() => { reload(); }, [reload, refreshKey]);

  if (error) return <div className="tree-error">tree: {error}</div>;

  return (
    <div className="file-tree">
      <div className="tree-head">FILES</div>
      <Tree
        data={data}
        openByDefault={false}
        width="100%"
        height={600}
        indent={14}
        rowHeight={24}
        disableDrag
        disableDrop
      >
        {({ node, style, dragHandle }) => (
          <div
            style={style}
            ref={dragHandle}
            className={`tree-row ${node.isSelected ? 'sel' : ''}`}
            onClick={() => {
              if (node.data.isDir) node.toggle();
              else onOpenFile(node.data.id);
            }}
          >
            <span className="tree-icon">
              {node.data.isDir ? (node.isOpen ? '▾' : '▸') : '·'}
            </span>
            <span className="tree-name">{node.data.name}</span>
          </div>
        )}
      </Tree>
    </div>
  );
}
