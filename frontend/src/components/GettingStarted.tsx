interface Step {
  done: boolean;
  title: string;
  detail: string;
  action: string;
  onGo: () => void;
}

interface Props {
  hasZones: boolean;
  hasVehicles: boolean;
  hasMoved: boolean;
  hasAlert: boolean;
  go: (target: 'geofences' | 'vehicles' | 'location' | 'live') => void;
  onDismiss: () => void;
}

/** Step-by-step checklist that ticks itself off as the user works through the demo flow. */
export default function GettingStarted({ hasZones, hasVehicles, hasMoved, hasAlert, go, onDismiss }: Props) {
  const steps: Step[] = [
    {
      done: hasZones,
      title: 'Draw a zone on the map',
      detail: 'Mark an area such as a delivery area or a no-entry zone.',
      action: 'Create zone',
      onGo: () => go('geofences'),
    },
    {
      done: hasVehicles,
      title: 'Add a vehicle',
      detail: 'Register a truck or car with its driver details.',
      action: 'Add vehicle',
      onGo: () => go('vehicles'),
    },
    {
      done: hasMoved,
      title: 'Move the vehicle',
      detail: 'Click on the map to place it. Move it into a zone and back out.',
      action: 'Move vehicle',
      onGo: () => go('location'),
    },
    {
      done: hasAlert,
      title: 'See the alert',
      detail: 'Entering a restricted zone (or any zone with an alert rule) raises a live alert.',
      action: 'Open alerts',
      onGo: () => go('live'),
    },
  ];
  const doneCount = steps.filter((s) => s.done).length;
  const next = steps.findIndex((s) => !s.done);

  return (
    <section className="space-y-3 rounded-lg border border-slate-200 bg-gradient-to-br from-white to-slate-50 p-3 shadow-sm">
      <div className="flex items-start justify-between gap-2">
        <div>
          <h2 className="text-sm font-semibold text-slate-900">👋 Getting started</h2>
          <p className="text-xs text-slate-500">
            {doneCount === steps.length ? 'All done! You have seen the full flow.' : `${doneCount} of ${steps.length} steps done`}
          </p>
        </div>
        <button onClick={onDismiss} className="text-xs text-slate-400 hover:text-slate-700" aria-label="Hide guide">
          Hide
        </button>
      </div>

      <div className="h-1.5 overflow-hidden rounded-full bg-slate-200">
        <div className="h-full rounded-full bg-green-500 transition-all" style={{ width: `${(doneCount / steps.length) * 100}%` }} />
      </div>

      <ol className="space-y-2">
        {steps.map((s, i) => (
          <li
            key={s.title}
            className={`flex items-center gap-3 rounded-md p-2 ${i === next ? 'bg-blue-50 ring-1 ring-blue-200' : ''}`}
          >
            <span
              className={`flex h-6 w-6 shrink-0 items-center justify-center rounded-full text-xs font-bold ${
                s.done ? 'bg-green-500 text-white' : i === next ? 'bg-blue-600 text-white' : 'bg-slate-200 text-slate-500'
              }`}
            >
              {s.done ? '✓' : i + 1}
            </span>
            <div className="min-w-0 flex-1">
              <div className={`text-sm font-medium ${s.done ? 'text-slate-400 line-through' : 'text-slate-800'}`}>{s.title}</div>
              {i === next && <div className="text-xs text-slate-500">{s.detail}</div>}
            </div>
            {i === next && (
              <button onClick={s.onGo} className="shrink-0 rounded-md bg-blue-600 px-2.5 py-1 text-xs font-medium text-white hover:bg-blue-700">
                {s.action} →
              </button>
            )}
          </li>
        ))}
      </ol>
    </section>
  );
}
