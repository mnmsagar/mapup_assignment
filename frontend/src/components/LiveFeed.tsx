import type { LiveAlert } from '../api/types';
import type { SocketStatus } from '../hooks/useAlertsSocket';
import { fmtCoord, formatTime, relativeTime } from '../lib/format';
import { CategoryBadge, EmptyState, EventBadge, PanelIntro } from './ui';

interface Props {
  alerts: LiveAlert[];
  onClear: () => void;
  onFocus: (a: LiveAlert) => void;
}

export function ConnectionPill({ status }: { status: SocketStatus }) {
  const map = {
    open: { dot: 'bg-green-500', text: 'Live' },
    connecting: { dot: 'bg-amber-400 animate-pulse', text: 'Connecting…' },
    reconnecting: { dot: 'bg-red-500 animate-pulse', text: 'Reconnecting…' },
  }[status];
  return (
    <span className="inline-flex items-center gap-1.5 rounded-full bg-white/10 px-2.5 py-1 text-xs font-medium" title="Real-time alert connection">
      <span className={`h-2 w-2 rounded-full ${map.dot}`} />
      {map.text}
    </span>
  );
}

export default function LiveFeed({ alerts, onClear, onFocus }: Props) {
  return (
    <div className="space-y-3">
      <PanelIntro icon="🔔" title="Live alerts">
        Alerts show up here instantly, with no page refresh, whenever a vehicle enters or leaves a zone you are watching.
        Red cards mean a vehicle entered a restricted zone. Click a card to highlight it on the map.
      </PanelIntro>
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold text-slate-800">Latest alerts</h3>
        {alerts.length > 0 && (
          <button onClick={onClear} className="text-xs text-slate-500 hover:text-slate-800">
            Clear
          </button>
        )}
      </div>
      {alerts.length === 0 ? (
        <EmptyState>
          No alerts yet. To get one, move a vehicle into a restricted zone, or create an alert rule and move a vehicle
          into that zone.
        </EmptyState>
      ) : (
        <ul className="space-y-2">
          {alerts.map((a) => (
            <AlertCard key={a.event_id} alert={a} onClick={() => onFocus(a)} />
          ))}
        </ul>
      )}
    </div>
  );
}

export function AlertCard({ alert: a, onClick }: { alert: LiveAlert; onClick?: () => void }) {
  const restricted = a.geofence.category === 'restricted_zone' && a.event_type === 'entry';
  return (
    <li>
      <button
        onClick={onClick}
        className={`w-full space-y-1 rounded-md border p-3 text-left text-sm transition hover:shadow ${
          restricted ? 'border-red-300 bg-red-50' : 'border-slate-200 bg-white'
        }`}
      >
        <div className="flex items-center justify-between gap-2">
          <span className="font-semibold text-slate-800">
            {restricted && '⚠️ '}
            {a.vehicle.vehicle_number}
          </span>
          <EventBadge type={a.event_type} />
        </div>
        <div className="flex flex-wrap items-center gap-1.5 text-slate-700">
          {a.event_type === 'entry' ? 'entered' : 'exited'} <span className="font-medium">{a.geofence.geofence_name}</span>
          <CategoryBadge category={a.geofence.category} />
        </div>
        <div className="flex justify-between text-[11px] text-slate-500">
          <span>
            {a.vehicle.driver_name} · {fmtCoord(a.location.latitude)}, {fmtCoord(a.location.longitude)}
          </span>
          <span title={formatTime(a.timestamp)}>{relativeTime(a.timestamp)}</span>
        </div>
      </button>
    </li>
  );
}
