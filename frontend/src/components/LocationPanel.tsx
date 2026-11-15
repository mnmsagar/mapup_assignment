import { useEffect, useState, type FormEvent } from 'react';
import toast from 'react-hot-toast';
import { api, errorMessage } from '../api/client';
import type { LatLng, LocationUpdateResult, Vehicle } from '../api/types';
import { localInputToISO, toLocalInput } from '../lib/format';
import type { MapMode } from './MapView';
import { Button, EmptyState, Field, Input, PanelIntro, Section, Select } from './ui';

interface Props {
  vehicles: Vehicle[];
  selectedVehicleId: string | null;
  onSelectVehicle: (id: string | null) => void;
  mode: MapMode;
  setMode: (m: MapMode) => void;
  pickedPoint: LatLng | null;
  setPickedPoint: (p: LatLng | null) => void;
  onUpdated: () => void;
}

export default function LocationPanel(props: Props) {
  const { vehicles, selectedVehicleId, mode, setMode, pickedPoint, setPickedPoint } = props;
  const [lat, setLat] = useState('');
  const [lng, setLng] = useState('');
  const [useNow, setUseNow] = useState(true);
  const [timestamp, setTimestamp] = useState(() => toLocalInput(new Date()));
  const [submitting, setSubmitting] = useState(false);
  const [result, setResult] = useState<LocationUpdateResult | null>(null);

  // Preselect the first vehicle so a single map click is enough to get going.
  const { onSelectVehicle } = props;
  useEffect(() => {
    if (!selectedVehicleId && vehicles.length > 0) onSelectVehicle(vehicles[0].id);
  }, [selectedVehicleId, vehicles, onSelectVehicle]);

  // Map clicks fill the inputs.
  useEffect(() => {
    if (pickedPoint) {
      setLat(String(pickedPoint[0]));
      setLng(String(pickedPoint[1]));
    }
  }, [pickedPoint]);

  // Typing coordinates moves the preview marker.
  const onCoordChange = (nextLat: string, nextLng: string) => {
    setLat(nextLat);
    setLng(nextLng);
    const a = Number(nextLat);
    const b = Number(nextLng);
    if (nextLat !== '' && nextLng !== '' && Number.isFinite(a) && Number.isFinite(b)) setPickedPoint([a, b]);
  };

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (!selectedVehicleId) return toast.error('Select a vehicle first.');
    const latitude = Number(lat);
    const longitude = Number(lng);
    if (lat === '' || !Number.isFinite(latitude) || latitude < -90 || latitude > 90)
      return toast.error('Latitude must be a number between -90 and 90.');
    if (lng === '' || !Number.isFinite(longitude) || longitude < -180 || longitude > 180)
      return toast.error('Longitude must be a number between -180 and 180.');
    const ts = useNow ? new Date().toISOString() : localInputToISO(timestamp);
    if (!ts) return toast.error('Pick a valid timestamp.');

    setSubmitting(true);
    try {
      const res = await api.updateLocation({ vehicle_id: selectedVehicleId, latitude, longitude, timestamp: ts });
      setResult(res);
      // Stay in pick mode so the next map click moves the vehicle again.
      setMode('pick');
      if (res.note) toast(res.note, { icon: 'ℹ️' });
      else
        toast.success(
          res.current_geofences.length
            ? `Vehicle moved. It is now inside: ${res.current_geofences.map((g) => g.geofence_name).join(', ')}`
            : 'Vehicle moved. It is not inside any zone.',
        );
      props.onUpdated();
    } catch (err) {
      toast.error(errorMessage(err));
    } finally {
      setSubmitting(false);
    }
  };

  const intro = (
    <PanelIntro icon="📍" title="Move a vehicle">
      There are no real trucks here, so you play the GPS device: choose where the vehicle is right now.
      <ol className="mt-1 list-decimal space-y-0.5 pl-4">
        <li>Pick a vehicle.</li>
        <li>Click anywhere on the map.</li>
        <li>
          Press <strong>Move vehicle here</strong>.
        </li>
      </ol>
      Move it from outside a zone to inside it to trigger an <strong>entry</strong>, then back out for an{' '}
      <strong>exit</strong>.
    </PanelIntro>
  );

  if (vehicles.length === 0) {
    return (
      <div className="space-y-4">
        {intro}
        <EmptyState>Add a vehicle first (🚚 Vehicles tab), then come back here to move it.</EmptyState>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {intro}
      <Section title="Where is the vehicle now?">
        <form onSubmit={submit} className="space-y-3">
          <Field label="Vehicle *">
            <Select value={selectedVehicleId ?? ''} onChange={(e) => props.onSelectVehicle(e.target.value || null)} required>
              <option value="">Select a vehicle…</option>
              {vehicles.map((v) => (
                <option key={v.id} value={v.id}>
                  {v.vehicle_number} — {v.driver_name}
                </option>
              ))}
            </Select>
          </Field>

          <div
            className={`rounded-md border border-dashed p-2 text-center text-xs ${
              pickedPoint ? 'border-green-300 bg-green-50 text-green-800' : 'border-violet-300 bg-violet-50 text-violet-800'
            }`}
          >
            {pickedPoint ? '✓ Point chosen on the map. You can click again to change it.' : '👆 Click on the map to choose the location'}
          </div>

          <div className="grid grid-cols-2 gap-3">
            <Field label="Latitude *">
              <Input value={lat} onChange={(e) => onCoordChange(e.target.value, lng)} inputMode="decimal" placeholder="37.7849" />
            </Field>
            <Field label="Longitude *">
              <Input value={lng} onChange={(e) => onCoordChange(lat, e.target.value)} inputMode="decimal" placeholder="-122.4194" />
            </Field>
          </div>

          <Button
            type="button"
            variant={mode === 'pick' ? 'primary' : 'secondary'}
            onClick={() => setMode(mode === 'pick' ? 'idle' : 'pick')}
            disabled={mode === 'draw'}
            className="w-full"
          >
            📍 {mode === 'pick' ? 'Map clicking is on (click here to turn off)' : 'Pick location on map'}
          </Button>

          <div className="space-y-2">
            <label className="flex items-center gap-2 text-sm text-slate-700">
              <input type="checkbox" checked={useNow} onChange={(e) => setUseNow(e.target.checked)} />
              Use current time (recommended)
            </label>
            {!useNow && (
              <Field label="Timestamp" hint="Your local time; sent as ISO 8601 UTC.">
                <Input type="datetime-local" value={timestamp} onChange={(e) => setTimestamp(e.target.value)} />
              </Field>
            )}
          </div>

          <Button type="submit" loading={submitting} disabled={!selectedVehicleId || lat === '' || lng === ''} className="w-full">
            🚚 Move vehicle here
          </Button>
        </form>
      </Section>

      {result && (
        <Section title="Result">
          <div className="space-y-2 rounded-md border border-slate-200 p-3 text-sm">
            <div className="text-slate-600">
              <span className="font-medium">{vehicles.find((v) => v.id === result.vehicle_id)?.vehicle_number ?? result.vehicle_id}</span> is now:
            </div>
            {result.current_geofences.length === 0 ? (
              <p className="rounded bg-slate-50 px-2 py-1.5 text-slate-600">Outside all zones</p>
            ) : (
              <ul className="space-y-1">
                {result.current_geofences.map((g) => (
                  <li key={g.geofence_id} className="flex items-center justify-between rounded bg-green-50 px-2 py-1.5">
                    <span>{g.geofence_name}</span>
                    <span className="text-xs font-semibold uppercase text-green-700">✓ inside</span>
                  </li>
                ))}
              </ul>
            )}
            <p className="text-xs text-slate-400">
              If this caused an entry or exit and an alert applies, it appears under 🔔 Live alerts.
            </p>
            {result.note && <p className="text-xs text-amber-700">{result.note}</p>}
          </div>
        </Section>
      )}
    </div>
  );
}
