import React, { useState, useEffect } from 'react';
import { Play, Pause, Square, Activity, Folder, FileText, CheckCircle, XCircle, Settings, Plus, Trash2, RefreshCw, Cpu, Clock, Terminal } from 'lucide-react';

interface PipelineStatus {
  total: number;
  processed: number;
  failed: number;
  summary: string;
  is_running: boolean;
  is_paused: boolean;
}

interface ModelInfo {
  name: string;
  url: string;
  status: 'Active' | 'Offline';
  is_default: boolean;
}

const App: React.FC = () => {
  const [src, setSrc] = useState('');
  const [dst, setDst] = useState('');
  const [status, setStatus] = useState<PipelineStatus | null>(null);
  const [isRunning, setIsRunning] = useState(false);
  const [isPaused, setIsPaused] = useState(false);
  const [history, setHistory] = useState<any[]>([]);
  const [models, setModels] = useState<ModelInfo[]>([]);
  const [newModelName, setNewModelName] = useState('');
  const [newModelURL, setNewModelURL] = useState('');
  const [workers, setWorkers] = useState(5);
  const [limit, setLimit] = useState(100000);

  const [activePage, setActivePage] = useState<'dashboard' | 'models'>('dashboard');

  const fetchModels = async () => {
    try {
      const res = await fetch('/v1/models');
      const data = await res.json();
      if (Array.isArray(data)) {
        setModels(data);
      }
    } catch (e) {
      console.error("Failed to fetch models", e);
    }
  };

  const fetchConfig = async () => {
    try {
      const res = await fetch('/v1/config');
      const data = await res.json();
      if (data.src) setSrc(data.src);
      if (data.dst) setDst(data.dst);
      if (data.workers) setWorkers(data.workers);
      if (data.limit) setLimit(data.limit);
    } catch (e) {
      console.error("Failed to fetch config", e);
    }
  };

  const fetchPipelineStatus = async (signal?: AbortSignal) => {
    try {
      const res = await fetch('/v1/pipeline/status', { signal });
      const data: PipelineStatus = await res.json();
      if (res.ok && data) {
        setStatus(data);
        setIsRunning(data.is_running);
        setIsPaused(data.is_paused);

        setHistory((prev: any[]) => {
          const newEntry = {
            time: new Date().toLocaleTimeString(),
            processed: typeof data.processed === 'number' ? data.processed : 0,
            failed: typeof data.failed === 'number' ? data.failed : 0
          };
          // Only add if it's a new timestamp or values changed significantly to keep history clean
          return [...prev.slice(-19), newEntry];
        });

        if (data.total > 0 && (data.processed + data.failed) >= data.total) {
          setIsRunning(false);
          setIsPaused(false); // Ensure paused state is reset if pipeline finishes
        }
      }
    } catch (e) {
      console.error("Failed to poll status", e);
      setIsRunning(false); // Stop polling if API fails
      setIsPaused(false);
    }
  };

  useEffect(() => {
    const controller = new AbortController();
    fetchModels();
    fetchConfig();
    fetchPipelineStatus(controller.signal);
    const interval = setInterval(fetchModels, 10000);
    return () => {
      controller.abort();
      clearInterval(interval);
    };
  }, []);

  useEffect(() => {
    let interval: any;
    const controller = new AbortController();
    if (isRunning) {
      interval = setInterval(() => fetchPipelineStatus(controller.signal), 1000);
    }
    return () => {
      controller.abort();
      if (interval) clearInterval(interval);
    };
  }, [isRunning]);

  const startPipeline = async () => {
    try {
      const selectedModel = models.find(m => m.status === 'Active');
      const res = await fetch('/v1/pipeline/start', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          src,
          dst,
          workers,
          limit,
          model: selectedModel?.name,
          model_url: selectedModel?.url
        })
      });
      if (res.ok) {
        setIsRunning(true);
        setIsPaused(false);
        fetchPipelineStatus(); // Fetch status immediately after starting
      }
    } catch (e) {
      alert("Failed to start pipeline");
    }
  };

  const stopPipeline = async () => {
    try {
      await fetch('/v1/pipeline/stop', { method: 'POST' });
      setIsRunning(false);
      setIsPaused(false);
      setStatus(null); // Clear status when stopped
      setHistory([]); // Clear history
    } catch (e) {
      console.error(e);
    }
  };

  const pausePipeline = async () => {
    try {
      await fetch('/v1/pipeline/pause', { method: 'POST' });
      setIsPaused(true);
    } catch (e) {
      console.error(e);
    }
  };

  const resumePipeline = async () => {
    try {
      await fetch('/v1/pipeline/resume', { method: 'POST' });
      setIsPaused(false);
    } catch (e) {
      console.error(e);
    }
  };

  const setModelDefault = async (name: string) => {
    if (isRunning && !isPaused) {
      alert("Pipeline is active! Please pause the pipeline before switching the default model to ensure smooth transition.");
      return;
    }
    try {
      await fetch('/v1/models/default', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name })
      });
      fetchModels();
    } catch (e) {
      console.error(e);
    }
  };

  const addModel = async () => {
    if (!newModelName || !newModelURL) return;
    try {
      await fetch('/v1/models', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: newModelName, url: newModelURL })
      });
      setNewModelName('');
      setNewModelURL('');
      fetchModels();
    } catch (e) {
      console.error(e);
    }
  };

  const removeModel = async (name: string) => {
    try {
      await fetch(`/v1/models?name=${encodeURIComponent(name)}`, { method: 'DELETE' });
      fetchModels();
    } catch (e) {
      console.error(e);
    }
  };

  const progress = status && status.total > 0 ? ((status.processed + status.failed) / status.total) * 100 : 0;

  const renderDashboard = () => (
    <div className="dashboard">
      <div className="page-header">
        <h2 className="page-title">Dashboard</h2>
      </div>

      {/* Control Panel */}
      <div className="card" style={{ gridColumn: 'span 4' }}>
        <div className="card-header">
          <div className="card-title"><Settings size={20} /> Configuration</div>
        </div>
        <div className="input-group">
          <label>Source Directory</label>
          <input
            value={src}
            onChange={e => setSrc(e.target.value)}
            placeholder="/Users/path/to/source"
            disabled={isRunning}
          />
        </div>
        <div className="input-group">
          <label>Destination Directory</label>
          <input
            value={dst}
            onChange={e => setDst(e.target.value)}
            placeholder="/Users/path/to/destination"
            disabled={isRunning}
          />
        </div>
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem' }}>
          <div className="input-group">
            <label>Concurrent Workers</label>
            <input
              type="number"
              value={workers}
              onChange={e => setWorkers(parseInt(e.target.value) || 5)}
              placeholder="5"
              disabled={isRunning}
              min="1"
              max="20"
            />
          </div>
          <div className="input-group">
            <label>Char Limit / File ({limit === 0 ? 'Auto' : limit})</label>
            <input
              type="number"
              value={limit}
              onChange={e => setLimit(parseInt(e.target.value) || 0)}
              placeholder="0 (Auto)"
              disabled={isRunning}
              min="0"
            />
          </div>
        </div>
        <div style={{ display: 'flex', gap: '1rem', marginTop: '1rem' }}>
          {isRunning ? (
            <div style={{ display: 'flex', gap: '0.5rem', flex: 1 }}>
              {isPaused ? (
                <button className="primary" onClick={resumePipeline} style={{ background: 'var(--success-color)', flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '0.5rem' }}>
                  <Play size={18} /> Resume
                </button>
              ) : (
                <button className="secondary" onClick={pausePipeline} style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '0.5rem' }}>
                  <Pause size={18} /> Pause
                </button>
              )}
              <button className="delete-btn" onClick={stopPipeline} style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '0.5rem' }}>
                <Square size={18} /> Stop
              </button>
            </div>
          ) : (
            <button onClick={startPipeline} style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '0.5rem' }}>
              <Play size={18} /> Start
            </button>
          )}
        </div>
      </div>

      {/* Status & Progress */}
      <div className="card" style={{ gridColumn: 'span 8' }}>
        <div className="card-header">
          <div className="card-title">
            <Activity size={20} />
            Pipeline Status
            <span className={`status-indicator ${isRunning ? (isPaused ? 'status-idle' : 'status-running') : 'status-idle'}`} style={{ marginLeft: '0.5rem' }}></span>
            <span style={{ fontSize: '0.9rem', fontWeight: 500 }}>
              {isRunning ? (isPaused ? 'Paused' : 'Running') : 'Idle'}
            </span>
          </div>
          <div style={{ fontSize: '0.85rem', color: 'var(--text-secondary)' }}>
            {isRunning ? (isPaused ? 'Paused' : 'Running') : 'Idle'}
          </div>
        </div>

        <div className="stats-grid">
          <div className="stat">
            <div className="stat-label">Total Files</div>
            <div className="stat-value">{status?.total || 0}</div>
          </div>
          <div className="stat">
            <div className="stat-label">Processed</div>
            <div className="stat-value" style={{ color: 'var(--success-color)' }}>{status?.processed || 0}</div>
          </div>
          <div className="stat">
            <div className="stat-label">Failed</div>
            <div className="stat-value" style={{ color: 'var(--error-color)' }}>{status?.failed || 0}</div>
          </div>
        </div>

        <div className="progress-bar-container">
          <div className="progress-bar-fill" style={{ width: `${progress}%` }}></div>
        </div>
        <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.85rem', color: 'var(--text-secondary)' }}>
          <span>Progress</span>
          <span>{progress.toFixed(1)}%</span>
        </div>
      </div>

      {/* Recent Activity Feed */}
      <div className="card" style={{ gridColumn: 'span 12', height: '300px', display: 'flex', flexDirection: 'column' }}>
        <div className="card-header">
          <div className="card-title"><Terminal size={20} /> Recent Activity</div>
        </div>
        <div style={{ flex: 1, overflowY: 'auto', padding: '0.5rem', background: 'rgba(0,0,0,0.2)', borderRadius: '4px', fontFamily: 'monospace', fontSize: '0.8rem' }}>
          {history.length > 0 ? (
            history.slice().reverse().map((h: any, i: number) => (
              <div key={i} style={{ padding: '0.2rem 0', borderBottom: '1px solid rgba(255,255,255,0.05)', display: 'flex', gap: '1rem' }}>
                <span style={{ color: 'var(--text-secondary)', width: '80px' }}>[{h.time}]</span>
                <span style={{ color: 'var(--success-color)' }}>Processed: {h.processed}</span>
                <span style={{ color: 'var(--error-color)' }}>Failed: {h.failed}</span>
                <span style={{ opacity: 0.5 }}>- Pipeline Heartbeat</span>
              </div>
            ))
          ) : (
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%', color: 'var(--text-secondary)' }}>
              Waiting for activity...
            </div>
          )}
        </div>
      </div>
    </div>
  );

  const renderModelManagement = () => (
    <div className="dashboard">
      <div className="page-header">
        <h2 className="page-title">Model Management</h2>
      </div>

      <div className="card" style={{ gridColumn: 'span 12' }}>
        <div className="card-header">
          <div className="card-title">
            <Cpu size={20} /> Model Pool
          </div>
          <button className="secondary" onClick={fetchModels} style={{ padding: '0.4rem 0.8rem' }}>
            <RefreshCw size={14} />
          </button>
        </div>

        <div className="model-pool">
          {models.map(m => (
            <div key={m.name} className="model-item">
              <div className="model-info" style={{ flex: 1 }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                  <span className={`status-indicator ${m.status === 'Active' ? 'status-running' : 'status-idle'}`} style={{ width: 8, height: 8 }}></span>
                  <span className="model-name" style={{ fontWeight: 'bold' }}>{m.name}</span>
                  {m.is_default && <span className="model-badge badge-default">Default</span>}
                  <span className={`model-badge ${m.status === 'Active' ? 'badge-active' : 'badge-offline'}`}>{m.status}</span>
                </div>
                <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginLeft: '1rem', marginTop: '0.25rem' }}>
                  {m.url}
                </div>
              </div>
              <div style={{ display: 'flex', gap: '0.5rem' }}>
                {!m.is_default && (
                  <button className="secondary" onClick={() => setModelDefault(m.name)} style={{ padding: '8px' }} title="Set as Default">
                    <CheckCircle size={18} />
                  </button>
                )}
                <button className="delete-btn" onClick={() => removeModel(m.name)} style={{ padding: '8px' }}>
                  <Trash2 size={18} />
                </button>
              </div>
            </div>
          ))}

          <div className="add-model-form card" style={{ marginTop: '2rem', display: 'flex', flexDirection: 'column', gap: '1rem', background: 'rgba(0,0,0,0.1)' }}>
            <div className="card-title" style={{ fontSize: '0.9rem' }}><Plus size={16} /> Add New Model</div>
            <div style={{ display: 'flex', gap: '1rem' }}>
              <div className="input-group" style={{ flex: 1, marginBottom: 0 }}>
                <label>Model Name</label>
                <input
                  value={newModelName}
                  onChange={e => setNewModelName(e.target.value)}
                  placeholder="e.g. gpt-4o"
                />
              </div>
              <div className="input-group" style={{ flex: 2, marginBottom: 0 }}>
                <label>API URL</label>
                <input
                  value={newModelURL}
                  onChange={e => setNewModelURL(e.target.value)}
                  placeholder="e.g. https://api.openai.com/v1"
                />
              </div>
            </div>
            <button onClick={addModel} style={{ alignSelf: 'flex-start', marginTop: '0.5rem' }}>
              Add Model to Pool
            </button>
          </div>
        </div>
      </div>
    </div>
  );

  return (
    <div className="app-container">
      <aside className="sidebar">
        <div className="sidebar-header">
          <h2>Docs Organiser</h2>
        </div>
        <nav className="nav-links">
          <div
            className={`nav-item ${activePage === 'dashboard' ? 'active' : ''}`}
            onClick={() => setActivePage('dashboard')}
          >
            <Activity size={20} /> Dashboard
          </div>
          <div
            className={`nav-item ${activePage === 'models' ? 'active' : ''}`}
            onClick={() => setActivePage('models')}
          >
            <Cpu size={20} /> Model Management
          </div>
        </nav>
      </aside>

      <main className="main-content">
        {activePage === 'dashboard' ? renderDashboard() : renderModelManagement()}
      </main>
    </div>
  );
};

export default App;
