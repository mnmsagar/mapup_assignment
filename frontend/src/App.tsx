import { useCallback, useEffect, useRef, useState } from 'react';
import toast, { Toaster } from 'react-hot-toast';
import { api, API_URL } from './api/client';
import type { LatLng, LiveAlert } from './api/types';
import AlertsPanel from './components/AlertsPanel';
import GeofencePanel from './components/GeofencePanel';
import GettingStarted from './components/GettingStarted';
import HistoryPanel from './components/HistoryPanel';
import LiveFeed, { AlertCard, ConnectionPill } from './components/LiveFeed';
import LocationPanel from './components/LocationPanel';
import MapView, { type MapMode } from './components/MapView';
import VehiclePanel from './components/VehiclePanel';
import { useAlertsSocket } from './hooks/useAlertsSocket';
import { useResource } from './hooks/useResource';

// Ordered to follow the natural flow: set up zones and vehicles, move vehicles, watch alerts.
const TABS = ['geofences', 'vehicles', 'location', 'alerts', 'live', 'history'] as const;
type Tab = (typeof TABS)[number];

const TAB_META: Record<Tab, { icon: string; label: string }> = {
  geofences: { icon: '🗺️', label: 'Zones' },
  vehicles: { icon: '🚚', label: 'Vehicles' },
  location: { icon: '📍', label: 'Move vehicle' },
  alerts: { icon: '⚙️', label: 'Alert rules' },
  live: { icon: '🔔', label: 'Live alerts' },
  history: { icon: '🕘', label: 'History' },
};

const GUIDE_KEY = 'geofence.guideHidden';

function readGuideHidden(): boolean {
  try {
    return localStorage.getItem(GUIDE_KEY) === '1';
  } catch {
    return false;
  }
}

export default function App() {
  const [tab, setTab] = useState<Tab>('geofences');
  const [guideHidden, setGuideHidden] = useState(readGuideHidden);
  const [mode, setMode] = useState<MapMode>('idle');
  const [draftPoints, setDraftPoints] = useState<LatLng[]>([]);
  const [pickedPoint, setPickedPoint] = useState<LatLng | null>(null);
  const [selectedVehicleId, setSelectedVehicleId] = useState<string | null>(null);
  const [highlightGeofenceId, setHighlightGeofenceId] = useState<string | null>(null);
  // Bumped when server-side state changes so dependent panels refetch.
  const [geofenceKey, setGeofenceKey] = useState(0);
  const [stateKey, setStateKey] = useState(0);
  const highlightTimer = useRef<ReturnType<typeof setTimeout>>();

  const geofences = useResource(() => api.listGeofences(), [], [geofenceKey]);
  const vehicles = useResource(() => api.listVehicles(), []);

  const highlight = useCallback((geofenceId: string) => {
    setHighlightGeofenceId(geofenceId);
    clearTimeout(highlightTimer.current);
    highlightTimer.current = setTimeout(() => setHighlightGeofenceId(null), 5000);
  }, []);

  const reloadVehicles = vehicles.reload;
  const onAlert = useCallback(
    (a: LiveAlert) => {
      toast.custom(
        (t) => (
          <ul className={`w-80 shadow-lg transition ${t.visible ? 'opacity-100' : 'opacity-0'}`} onClick={() => toast.dismiss(t.id)}>
            <AlertCard alert={a} />
          </ul>
        ),
        { duration: 6000, position: 'top-right' },
      );
      highlight(a.geofence.geofence_id);
      setStateKey((k) => k + 1);
      void reloadVehicles();
    },
    [highlight, reloadVehicles],
  );

  const socket = useAlertsSocket(onAlert);

  useEffect(() => {
    if (tab === 'live') socket.markRead();
  }, [tab, socket.alerts, socket.markRead]);

  const changeTab = (next: Tab) => {
    setTab(next);
    // On "Move vehicle" the map is ready to click straight away.
    setMode(next === 'location' ? 'pick' : 'idle');
    if (next !== 'geofences') setDraftPoints([]);
    if (next !== 'location') setPickedPoint(null);
  };

  const onMapClick = (p: LatLng) => {
    if (mode === 'draw') setDraftPoints([...draftPoints, p]);
    else if (mode === 'pick') setPickedPoint(p);
  };

  return (
    <div className="flex h-screen flex-col bg-slate-100 text-slate-900">
      <Toaster position="bottom-center" toastOptions={{ duration: 4000, style: { fontSize: 14 } }} />

      <header className="flex items-center justify-between gap-3 bg-slate-900 px-4 py-3 text-white">
        <div className="flex items-center gap-2">
          <img src="/favicon.svg" alt="" className="h-6 w-6" />
          <div>
            <h1 className="text-base font-semibold leading-tight sm:text-lg">Geofence Control</h1>
            <p className="hidden text-xs text-slate-400 sm:block">Track vehicles and get alerts when they enter or leave zones</p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          {guideHidden && (
            <button
              onClick={() => {
                setGuideHidden(false);
                try {
                  localStorage.removeItem(GUIDE_KEY);
                } catch {
                  /* storage unavailable */
                }
              }}
              className="rounded-full px-2.5 py-1 text-xs font-medium text-slate-300 hover:bg-white/10 hover:text-white"
            >
              ❓ Guide
            </button>
          )}
          <button
            onClick={() => changeTab('live')}
            className="relative rounded-full px-2 py-1 text-sm hover:bg-white/10"
            title="Real-time alerts"
            aria-label={`${socket.unread} unread alerts`}
          >
            🔔
            {socket.unread > 0 && (
              <span className="absolute -right-1 -top-1 min-w-[18px] rounded-full bg-red-500 px-1 text-center text-[11px] font-bold">
                {socket.unread > 99 ? '99+' : socket.unread}
              </span>
            )}
          </button>
          <ConnectionPill status={socket.status} />
        </div>
      </header>

      <main className="flex min-h-0 flex-1 flex-col lg:flex-row">
        <div className="relative h-[45vh] shrink-0 lg:h-auto lg:flex-1">
          {geofences.error && (
            <div className="absolute right-3 top-3 z-[1100] max-w-sm rounded-md bg-red-600 px-3 py-2 text-sm text-white shadow">
              Can't load data from {API_URL}. Is the backend running?{' '}
              <button onClick={geofences.reload} className="font-semibold underline">
                Retry
              </button>
            </div>
          )}
          <MapView
            geofences={geofences.data}
            vehicles={vehicles.data}
            mode={mode}
            draftPoints={draftPoints}
            pickedPoint={pickedPoint}
            selectedVehicleId={selectedVehicleId}
            highlightGeofenceId={highlightGeofenceId}
            onMapClick={onMapClick}
            onSelectVehicle={(id) => {
              setSelectedVehicleId(id);
              if (tab !== 'location') changeTab('vehicles');
            }}
          />
        </div>

        <aside className="flex min-h-0 flex-1 flex-col border-l border-slate-200 bg-white lg:w-[440px] lg:flex-none">
          <nav className="flex shrink-0 overflow-x-auto border-b border-slate-200" role="tablist">
            {TABS.map((t) => (
              <button
                key={t}
                role="tab"
                aria-selected={tab === t}
                onClick={() => changeTab(t)}
                className={`relative flex shrink-0 flex-col items-center gap-0.5 whitespace-nowrap px-3 py-2 text-xs font-medium transition-colors ${
                  tab === t ? 'border-b-2 border-blue-600 bg-blue-50/50 text-blue-700' : 'text-slate-500 hover:bg-slate-50 hover:text-slate-800'
                }`}
              >
                <span className="text-base leading-none" aria-hidden>
                  {TAB_META[t].icon}
                </span>
                {TAB_META[t].label}
                {t === 'live' && socket.unread > 0 && tab !== 'live' && (
                  <span className="ml-1 rounded-full bg-red-500 px-1.5 text-[10px] font-bold text-white">{socket.unread}</span>
                )}
              </button>
            ))}
          </nav>

          <div className="min-h-0 flex-1 space-y-4 overflow-y-auto p-4">
            {!guideHidden && (
              <GettingStarted
                hasZones={geofences.data.length > 0}
                hasVehicles={vehicles.data.length > 0}
                hasMoved={vehicles.data.some((v) => v.current_location)}
                hasAlert={socket.alerts.length > 0}
                go={changeTab}
                onDismiss={() => {
                  setGuideHidden(true);
                  try {
                    localStorage.setItem(GUIDE_KEY, '1');
                  } catch {
                    /* storage unavailable */
                  }
                }}
              />
            )}
            {tab === 'geofences' && (
              <GeofencePanel
                mode={mode}
                setMode={setMode}
                draftPoints={draftPoints}
                setDraftPoints={setDraftPoints}
                refreshKey={geofenceKey}
                onCreated={() => setGeofenceKey((k) => k + 1)}
              />
            )}
            {tab === 'vehicles' && (
              <VehiclePanel
                vehicles={vehicles.data}
                loading={vehicles.loading}
                error={vehicles.error}
                reload={vehicles.reload}
                selectedVehicleId={selectedVehicleId}
                onSelect={setSelectedVehicleId}
                statusKey={stateKey}
              />
            )}
            {tab === 'location' && (
              <LocationPanel
                vehicles={vehicles.data}
                selectedVehicleId={selectedVehicleId}
                onSelectVehicle={setSelectedVehicleId}
                mode={mode}
                setMode={setMode}
                pickedPoint={pickedPoint}
                setPickedPoint={setPickedPoint}
                onUpdated={() => {
                  setStateKey((k) => k + 1);
                  void vehicles.reload();
                }}
              />
            )}
            {tab === 'alerts' && <AlertsPanel geofences={geofences.data} vehicles={vehicles.data} />}
            {tab === 'live' && (
              <LiveFeed
                alerts={socket.alerts}
                onClear={socket.clear}
                onFocus={(a) => {
                  highlight(a.geofence.geofence_id);
                  setSelectedVehicleId(a.vehicle.vehicle_id);
                }}
              />
            )}
            {tab === 'history' && <HistoryPanel geofences={geofences.data} vehicles={vehicles.data} refreshKey={stateKey} />}
          </div>
        </aside>
      </main>
    </div>
  );
}
