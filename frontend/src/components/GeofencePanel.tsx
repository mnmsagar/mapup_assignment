import { useEffect, useState, type FormEvent } from 'react';
import toast from 'react-hot-toast';
import { api, errorMessage } from '../api/client';
import { CATEGORIES, type Category, type LatLng } from '../api/types';
import { useResource } from '../hooks/useResource';
import { CATEGORY_STYLE, formatTime } from '../lib/format';
import type { MapMode } from './MapView';
import { Button, CategoryBadge, EmptyState, ErrorBox, Field, Input, LoadingRows, Section, Select } from './ui';

interface Props {
  mode: MapMode;
  setMode: (m: MapMode) => void;
  draftPoints: LatLng[];
  setDraftPoints: (p: LatLng[]) => void;
  onCreated: () => void;
  refreshKey: number;
}

const pointsToText = (pts: LatLng[]) => pts.map(([lat, lng]) => `${lat}, ${lng}`).join('\n');

/** Parses one "lat, lng" pair per line (a JSON [[lat,lng],...] array also works). */
function parsePoints(text: string): { points: LatLng[]; error?: string } {
  const trimmed = text.trim();
  if (!trimmed) return { points: [] };
  if (trimmed.startsWith('[')) {
    try {
      const arr = JSON.parse(trimmed);
      if (Array.isArray(arr) && arr.every((p) => Array.isArray(p) && p.length === 2 && p.every(Number.isFinite))) {
        return { points: arr as LatLng[] };
      }
    } catch {
      /* fall through */
    }
    return { points: [], error: 'Invalid JSON — expected [[lat, lng], ...]' };
  }
  const points: LatLng[] = [];
  const lines = trimmed.split('\n');
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i].trim();
    if (!line) continue;
    const parts = line.split(/[,\s]+/).filter(Boolean).map(Number);
    if (parts.length !== 2 || !parts.every(Number.isFinite)) {
      return { points: [], error: `Line ${i + 1}: expected "latitude, longitude"` };
    }
    points.push([parts[0], parts[1]]);
  }
  return { points };
}

const isClosed = (pts: LatLng[]) =>
  pts.length > 1 && pts[0][0] === pts[pts.length - 1][0] && pts[0][1] === pts[pts.length - 1][1];

export default function GeofencePanel({ mode, setMode, draftPoints, setDraftPoints, onCreated, refreshKey }: Props) {
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [category, setCategory] = useState<Category>('delivery_zone');
  const [coordText, setCoordText] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [filter, setFilter] = useState<Category | ''>('');

  const list = useResource(() => api.listGeofences(filter), [], [filter, refreshKey]);
  const drawing = mode === 'draw';

  // Points clicked on the map flow into the textarea.
  useEffect(() => {
    if (drawing) setCoordText(pointsToText(draftPoints));
  }, [draftPoints, drawing]);

  const parsed = parsePoints(coordText);

  const startDrawing = () => {
    setDraftPoints([]);
    setCoordText('');
    setMode('draw');
  };

  const finishDrawing = () => {
    setMode('idle');
    if (draftPoints.length >= 3 && !isClosed(draftPoints)) {
      const closed = [...draftPoints, draftPoints[0]];
      setDraftPoints(closed);
      setCoordText(pointsToText(closed));
    }
  };

  const reset = () => {
    setName('');
    setDescription('');
    setCoordText('');
    setDraftPoints([]);
    setMode('idle');
  };

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (parsed.error) return toast.error(parsed.error);
    let coordinates = parsed.points;
    if (coordinates.length >= 3 && !isClosed(coordinates)) coordinates = [...coordinates, coordinates[0]];
    if (coordinates.length < 4) return toast.error('A geofence needs at least 3 distinct points.');

    setSubmitting(true);
    try {
      const res = await api.createGeofence({ name: name.trim(), description: description.trim(), category, coordinates });
      toast.success(`Geofence "${res.name}" created (${res.id})`);
      reset();
      onCreated();
    } catch (err) {
      toast.error(errorMessage(err));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="space-y-6">
      <Section title="Create geofence">
        <form onSubmit={submit} className="space-y-3">
          <Field label="Name *">
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="Downtown Delivery Zone" required />
          </Field>
          <Field label="Description">
            <Input value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Optional" />
          </Field>
          <Field label="Category *">
            <Select value={category} onChange={(e) => setCategory(e.target.value as Category)}>
              {CATEGORIES.map((c) => (
                <option key={c} value={c}>
                  {CATEGORY_STYLE[c].label}
                </option>
              ))}
            </Select>
          </Field>

          <Field
            label="Boundary *"
            hint={
              parsed.error ? (
                <span className="text-red-600">{parsed.error}</span>
              ) : (
                `${parsed.points.length} point(s). One "lat, lng" per line — the polygon is closed automatically.`
              )
            }
          >
            <textarea
              value={coordText}
              onChange={(e) => {
                setCoordText(e.target.value);
                setDraftPoints(parsePoints(e.target.value).points);
              }}
              disabled={drawing}
              rows={5}
              placeholder={'37.7749, -122.4194\n37.7849, -122.4194\n37.7849, -122.4094'}
              className="w-full rounded-md border border-slate-300 px-2.5 py-1.5 font-mono text-xs shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 disabled:bg-slate-50"
            />
          </Field>

          <div className="flex flex-wrap gap-2">
            {drawing ? (
              <>
                <Button type="button" variant="secondary" onClick={() => setDraftPoints(draftPoints.slice(0, -1))} disabled={!draftPoints.length}>
                  Undo point
                </Button>
                <Button type="button" onClick={finishDrawing} disabled={draftPoints.length < 3}>
                  Finish drawing
                </Button>
              </>
            ) : (
              <Button type="button" variant="secondary" onClick={startDrawing} disabled={mode === 'pick'}>
                ✏️ Draw on map
              </Button>
            )}
            <Button type="button" variant="ghost" onClick={reset}>
              Clear
            </Button>
            <Button type="submit" loading={submitting} disabled={drawing} className="ml-auto">
              Create
            </Button>
          </div>
        </form>
      </Section>

      <Section
        title="Geofences"
        action={
          <Select value={filter} onChange={(e) => setFilter(e.target.value as Category | '')} className="!w-auto !py-1 text-xs">
            <option value="">All categories</option>
            {CATEGORIES.map((c) => (
              <option key={c} value={c}>
                {CATEGORY_STYLE[c].label}
              </option>
            ))}
          </Select>
        }
      >
        {list.error ? (
          <ErrorBox message={list.error} onRetry={list.reload} />
        ) : list.loading && list.data.length === 0 ? (
          <LoadingRows />
        ) : list.data.length === 0 ? (
          <EmptyState>No geofences{filter && ' in this category'} yet.</EmptyState>
        ) : (
          <ul className="divide-y divide-slate-100 rounded-md border border-slate-200">
            {list.data.map((g) => (
              <li key={g.id} className="space-y-1 p-3">
                <div className="flex items-center justify-between gap-2">
                  <span className="truncate font-medium text-slate-800">{g.name}</span>
                  <CategoryBadge category={g.category} />
                </div>
                {g.description && <p className="text-xs text-slate-600">{g.description}</p>}
                <p className="text-[11px] text-slate-400">
                  {g.id} · {g.coordinates.length - 1} vertices · {formatTime(g.created_at)}
                </p>
              </li>
            ))}
          </ul>
        )}
      </Section>
    </div>
  );
}
