import { useState, type FormEvent } from 'react';
import toast from 'react-hot-toast';
import { api, errorMessage } from '../api/client';
import { EVENT_TYPES, type AlertEventType, type Geofence, type Vehicle } from '../api/types';
import { useResource } from '../hooks/useResource';
import { formatTime } from '../lib/format';
import { Button, EmptyState, ErrorBox, EventBadge, Field, LoadingRows, PanelIntro, Section, Select } from './ui';

interface Props {
  geofences: Geofence[];
  vehicles: Vehicle[];
}

const EVENT_LABEL: Record<AlertEventType, string> = { entry: 'Enters', exit: 'Leaves', both: 'Enters or leaves' };

export default function AlertsPanel({ geofences, vehicles }: Props) {
  const [geofenceId, setGeofenceId] = useState('');
  const [vehicleId, setVehicleId] = useState('');
  const [eventType, setEventType] = useState<AlertEventType>('entry');
  const [submitting, setSubmitting] = useState(false);
  const [filterGeofence, setFilterGeofence] = useState('');
  const [filterVehicle, setFilterVehicle] = useState('');
  const [deleting, setDeleting] = useState<string | null>(null);

  const rules = useResource(
    () => api.listAlerts({ geofence_id: filterGeofence, vehicle_id: filterVehicle }),
    [],
    [filterGeofence, filterVehicle],
  );

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (!geofenceId) return toast.error('Choose a geofence to monitor.');
    setSubmitting(true);
    try {
      await api.configureAlert({ geofence_id: geofenceId, vehicle_id: vehicleId || undefined, event_type: eventType });
      toast.success('Alert rule saved');
      rules.reload();
    } catch (err) {
      toast.error(errorMessage(err));
    } finally {
      setSubmitting(false);
    }
  };

  const remove = async (id: string) => {
    if (!window.confirm(`Delete alert rule ${id}?`)) return;
    setDeleting(id);
    try {
      await api.deleteAlert(id);
      toast.success(`Alert rule ${id} deleted`);
      rules.reload();
    } catch (err) {
      toast.error(errorMessage(err));
    } finally {
      setDeleting(null);
    }
  };

  return (
    <div className="space-y-6">
      <PanelIntro icon="⚙️" title="Alert rules">
        A rule says <em>when</em> you want to be notified. Example: “Tell me when <strong>any vehicle</strong> enters{' '}
        <strong>Downtown Delivery Zone</strong>.”
        <br />
        Restricted zones always alert on entry, even without a rule.
      </PanelIntro>

      <Section title="Create a rule">
        {geofences.length === 0 ? (
          <EmptyState>Create a zone first (🗺️ Zones tab).</EmptyState>
        ) : (
          <form onSubmit={submit} className="space-y-3">
            <Field label="Which zone? *">
              <Select value={geofenceId} onChange={(e) => setGeofenceId(e.target.value)} required>
                <option value="">Select a zone…</option>
                {geofences.map((g) => (
                  <option key={g.id} value={g.id}>
                    {g.name} ({g.id})
                  </option>
                ))}
              </Select>
            </Field>
            <Field label="Which vehicle?" hint="Leave as “All vehicles” to alert for any vehicle.">
              <Select value={vehicleId} onChange={(e) => setVehicleId(e.target.value)}>
                <option value="">All vehicles</option>
                {vehicles.map((v) => (
                  <option key={v.id} value={v.id}>
                    {v.vehicle_number}
                  </option>
                ))}
              </Select>
            </Field>
            <Field label="Alert me when the vehicle… *">
              <div className="grid grid-cols-3 gap-1 rounded-md bg-slate-100 p-1">
                {EVENT_TYPES.map((t) => (
                  <button
                    key={t}
                    type="button"
                    onClick={() => setEventType(t)}
                    className={`rounded px-2 py-1 text-xs font-medium ${eventType === t ? 'bg-white text-slate-900 shadow' : 'text-slate-500'}`}
                  >
                    {EVENT_LABEL[t]}
                  </button>
                ))}
              </div>
            </Field>
            <div className="flex justify-end">
              <Button type="submit" loading={submitting}>
                Save rule
              </Button>
            </div>
          </form>
        )}
      </Section>

      <Section title="Your rules">
        <div className="grid grid-cols-2 gap-2">
          <Select value={filterGeofence} onChange={(e) => setFilterGeofence(e.target.value)} className="text-xs">
            <option value="">Any zone</option>
            {geofences.map((g) => (
              <option key={g.id} value={g.id}>
                {g.name}
              </option>
            ))}
          </Select>
          <Select value={filterVehicle} onChange={(e) => setFilterVehicle(e.target.value)} className="text-xs">
            <option value="">Any vehicle</option>
            {vehicles.map((v) => (
              <option key={v.id} value={v.id}>
                {v.vehicle_number}
              </option>
            ))}
          </Select>
        </div>

        {rules.error ? (
          <ErrorBox message={rules.error} onRetry={rules.reload} />
        ) : rules.loading && rules.data.length === 0 ? (
          <LoadingRows />
        ) : rules.data.length === 0 ? (
          <EmptyState>No rules yet. Create one above.</EmptyState>
        ) : (
          <ul className="divide-y divide-slate-100 rounded-md border border-slate-200">
            {rules.data.map((r) => (
              <li key={r.alert_id} className="flex items-center justify-between gap-2 p-3">
                <div className="min-w-0 space-y-0.5">
                  <div className="flex items-center gap-2">
                    <span className="truncate font-medium text-slate-800">{r.geofence_name}</span>
                    <EventBadge type={r.event_type} />
                  </div>
                  <div className="text-xs text-slate-500">{r.vehicle_number ?? 'All vehicles'}</div>
                  <div className="text-[11px] text-slate-400">
                    {r.alert_id} · {formatTime(r.created_at)}
                  </div>
                </div>
                <Button variant="danger" onClick={() => remove(r.alert_id)} loading={deleting === r.alert_id} className="!px-2 !py-1 text-xs">
                  Delete
                </Button>
              </li>
            ))}
          </ul>
        )}
      </Section>
    </div>
  );
}
