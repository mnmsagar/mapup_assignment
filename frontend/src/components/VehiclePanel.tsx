import { useEffect, useState, type FormEvent } from 'react';
import toast from 'react-hot-toast';
import { api, errorMessage } from '../api/client';
import type { Vehicle, VehicleStatus } from '../api/types';
import { fmtCoord, relativeTime } from '../lib/format';
import { Button, CategoryBadge, EmptyState, ErrorBox, Field, Input, LoadingRows, PanelIntro, Section, Select, Spinner } from './ui';

interface Props {
  vehicles: Vehicle[];
  loading: boolean;
  error: string | null;
  reload: () => void;
  selectedVehicleId: string | null;
  onSelect: (id: string | null) => void;
  /** Bumps whenever a location update or alert may have changed vehicle state. */
  statusKey: number;
}

const VEHICLE_TYPES = ['truck', 'van', 'car', 'motorcycle', 'bus'];

export default function VehiclePanel({ vehicles, loading, error, reload, selectedVehicleId, onSelect, statusKey }: Props) {
  const [form, setForm] = useState({ vehicle_number: '', driver_name: '', vehicle_type: 'truck', phone: '' });
  const [submitting, setSubmitting] = useState(false);

  const set = (k: keyof typeof form) => (e: { target: { value: string } }) => setForm({ ...form, [k]: e.target.value });

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    try {
      const res = await api.createVehicle(form);
      toast.success(`Vehicle ${res.vehicle_number} added`);
      setForm({ vehicle_number: '', driver_name: '', vehicle_type: form.vehicle_type, phone: '' });
      reload();
    } catch (err) {
      toast.error(errorMessage(err));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="space-y-6">
      <PanelIntro icon="🚚" title="Vehicles">
        Add the trucks or cars you want to track. Click a vehicle in the list below to see where it is and which zones
        it is inside.
      </PanelIntro>

      <Section title="Add a vehicle">
        <form onSubmit={submit} className="grid grid-cols-2 gap-3">
          <Field label="Vehicle number *">
            <Input value={form.vehicle_number} onChange={set('vehicle_number')} placeholder="KA-01-AB-1234" required />
          </Field>
          <Field label="Type *">
            <Select value={form.vehicle_type} onChange={set('vehicle_type')}>
              {VEHICLE_TYPES.map((t) => (
                <option key={t} value={t}>
                  {t[0].toUpperCase() + t.slice(1)}
                </option>
              ))}
            </Select>
          </Field>
          <Field label="Driver name *">
            <Input value={form.driver_name} onChange={set('driver_name')} placeholder="John Doe" required />
          </Field>
          <Field label="Phone *">
            <Input value={form.phone} onChange={set('phone')} placeholder="+1234567890" type="tel" required />
          </Field>
          <div className="col-span-2 flex justify-end">
            <Button type="submit" loading={submitting}>
              Add vehicle
            </Button>
          </div>
        </form>
      </Section>

      <Section
        title={`Your vehicles (${vehicles.length})`}
        action={
          <Button variant="ghost" onClick={reload} loading={loading} className="!px-2 !py-1 text-xs">
            Refresh
          </Button>
        }
      >
        {error ? (
          <ErrorBox message={error} onRetry={reload} />
        ) : loading && vehicles.length === 0 ? (
          <LoadingRows />
        ) : vehicles.length === 0 ? (
          <EmptyState>No vehicles yet. Add one above.</EmptyState>
        ) : (
          <ul className="divide-y divide-slate-100 rounded-md border border-slate-200">
            {vehicles.map((v) => (
              <VehicleRow
                key={v.id}
                vehicle={v}
                expanded={v.id === selectedVehicleId}
                onToggle={() => onSelect(v.id === selectedVehicleId ? null : v.id)}
                statusKey={statusKey}
              />
            ))}
          </ul>
        )}
      </Section>
    </div>
  );
}

function VehicleRow({ vehicle: v, expanded, onToggle, statusKey }: { vehicle: Vehicle; expanded: boolean; onToggle: () => void; statusKey: number }) {
  const [status, setStatus] = useState<VehicleStatus | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!expanded) return;
    let cancelled = false;
    setLoading(true);
    api
      .vehicleStatus(v.id)
      .then((s) => !cancelled && (setStatus(s), setError(null)))
      .catch((err) => !cancelled && setError(errorMessage(err)))
      .finally(() => !cancelled && setLoading(false));
    return () => {
      cancelled = true;
    };
  }, [expanded, v.id, statusKey]);

  return (
    <li className={expanded ? 'bg-violet-50/60' : ''}>
      <button onClick={onToggle} className="flex w-full items-center justify-between gap-2 p-3 text-left hover:bg-slate-50">
        <div className="min-w-0">
          <div className="font-medium text-slate-800">{v.vehicle_number}</div>
          <div className="truncate text-xs text-slate-500">
            {v.driver_name} · {v.vehicle_type} · {v.phone}
          </div>
        </div>
        <div className="shrink-0 text-right text-[11px] text-slate-400">
          {v.current_location ? `seen ${relativeTime(v.current_location.timestamp)}` : 'no location yet'}
          <div>{expanded ? '▲' : '▼'}</div>
        </div>
      </button>

      {expanded && (
        <div className="space-y-2 px-3 pb-3 text-xs">
          {loading && !status ? (
            <div className="flex items-center gap-2 text-slate-500">
              <Spinner className="h-3 w-3" /> Loading status…
            </div>
          ) : error ? (
            <ErrorBox message={error} />
          ) : status ? (
            <>
              <div className="text-slate-600">
                <span className="font-medium">Location: </span>
                {status.current_location
                  ? `${fmtCoord(status.current_location.latitude)}, ${fmtCoord(status.current_location.longitude)}`
                  : 'not placed yet. Use 📍 Move vehicle to put it on the map'}
              </div>
              <div className="flex flex-wrap items-center gap-1">
                <span className="font-medium text-slate-600">Inside:</span>
                {status.current_geofences.length === 0 ? (
                  <span className="text-slate-400">no zones</span>
                ) : (
                  status.current_geofences.map((g) => (
                    <span key={g.geofence_id} className="inline-flex items-center gap-1 rounded bg-white px-1.5 py-0.5 shadow-sm">
                      {g.geofence_name} <CategoryBadge category={g.category} />
                    </span>
                  ))
                )}
              </div>
              <div className="text-[11px] text-slate-400">{v.id}</div>
            </>
          ) : null}
        </div>
      )}
    </li>
  );
}
