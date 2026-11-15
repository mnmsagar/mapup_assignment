import { useState } from 'react';
import { api } from '../api/client';
import type { Geofence, Vehicle } from '../api/types';
import { useResource } from '../hooks/useResource';
import { fmtCoord, formatTime, localInputToISO } from '../lib/format';
import { Button, EmptyState, ErrorBox, EventBadge, Field, Input, LoadingRows, Select, Spinner } from './ui';

interface Props {
  geofences: Geofence[];
  vehicles: Vehicle[];
  refreshKey: number;
}

const PAGE_SIZES = [10, 25, 50, 100];

export default function HistoryPanel({ geofences, vehicles, refreshKey }: Props) {
  const [vehicleId, setVehicleId] = useState('');
  const [geofenceId, setGeofenceId] = useState('');
  const [start, setStart] = useState('');
  const [end, setEnd] = useState('');
  const [pageSize, setPageSize] = useState(25);
  const [page, setPage] = useState(0);

  const history = useResource(
    () =>
      api.violationHistory({
        vehicle_id: vehicleId,
        geofence_id: geofenceId,
        start_date: localInputToISO(start),
        end_date: localInputToISO(end),
        limit: pageSize,
        offset: page * pageSize,
      }),
    { violations: [], total_count: 0 },
    [vehicleId, geofenceId, start, end, pageSize, page, refreshKey],
  );

  // Any filter change goes back to the first page.
  const onFilter = <T,>(setter: (v: T) => void) => (v: T) => {
    setter(v);
    setPage(0);
  };

  const { violations, total_count } = history.data;
  const pages = Math.max(1, Math.ceil(total_count / pageSize));
  const from = total_count === 0 ? 0 : page * pageSize + 1;
  const to = Math.min(total_count, (page + 1) * pageSize);
  const filtered = vehicleId || geofenceId || start || end;

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-2">
        <Field label="Vehicle">
          <Select value={vehicleId} onChange={(e) => onFilter(setVehicleId)(e.target.value)}>
            <option value="">All vehicles</option>
            {vehicles.map((v) => (
              <option key={v.id} value={v.id}>
                {v.vehicle_number}
              </option>
            ))}
          </Select>
        </Field>
        <Field label="Geofence">
          <Select value={geofenceId} onChange={(e) => onFilter(setGeofenceId)(e.target.value)}>
            <option value="">All geofences</option>
            {geofences.map((g) => (
              <option key={g.id} value={g.id}>
                {g.name}
              </option>
            ))}
          </Select>
        </Field>
        <Field label="From">
          <Input type="datetime-local" value={start} onChange={(e) => onFilter(setStart)(e.target.value)} />
        </Field>
        <Field label="To">
          <Input type="datetime-local" value={end} onChange={(e) => onFilter(setEnd)(e.target.value)} />
        </Field>
      </div>
      {filtered && (
        <button
          className="text-xs text-blue-600 hover:underline"
          onClick={() => {
            setVehicleId('');
            setGeofenceId('');
            setStart('');
            setEnd('');
            setPage(0);
          }}
        >
          Reset filters
        </button>
      )}

      {history.error ? (
        <ErrorBox message={history.error} onRetry={history.reload} />
      ) : history.loading && violations.length === 0 ? (
        <LoadingRows />
      ) : violations.length === 0 ? (
        <EmptyState>No geofence events{filtered ? ' match these filters' : ' recorded yet'}.</EmptyState>
      ) : (
        <div className="relative overflow-x-auto rounded-md border border-slate-200">
          {history.loading && (
            <div className="absolute inset-0 flex items-center justify-center bg-white/60">
              <Spinner className="h-5 w-5 text-blue-600" />
            </div>
          )}
          <table className="w-full text-left text-xs">
            <thead className="bg-slate-50 text-slate-500">
              <tr>
                <th className="px-2 py-2 font-medium">Time</th>
                <th className="px-2 py-2 font-medium">Vehicle</th>
                <th className="px-2 py-2 font-medium">Geofence</th>
                <th className="px-2 py-2 font-medium">Event</th>
                <th className="hidden px-2 py-2 font-medium sm:table-cell">Location</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {violations.map((v) => (
                <tr key={v.id} className="hover:bg-slate-50">
                  <td className="whitespace-nowrap px-2 py-2 text-slate-600">{formatTime(v.timestamp)}</td>
                  <td className="px-2 py-2 font-medium text-slate-800">{v.vehicle_number}</td>
                  <td className="px-2 py-2 text-slate-700">{v.geofence_name}</td>
                  <td className="px-2 py-2">
                    <EventBadge type={v.event_type} />
                  </td>
                  <td className="hidden whitespace-nowrap px-2 py-2 font-mono text-slate-500 sm:table-cell">
                    {fmtCoord(v.latitude)}, {fmtCoord(v.longitude)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-slate-600">
        <span>
          {from}–{to} of {total_count}
        </span>
        <div className="flex items-center gap-2">
          <Select value={pageSize} onChange={(e) => onFilter(setPageSize)(Number(e.target.value))} className="!w-auto !py-1 text-xs">
            {PAGE_SIZES.map((n) => (
              <option key={n} value={n}>
                {n} / page
              </option>
            ))}
          </Select>
          <Button variant="secondary" className="!px-2 !py-1 text-xs" disabled={page === 0} onClick={() => setPage(page - 1)}>
            ‹ Prev
          </Button>
          <span>
            {page + 1} / {pages}
          </span>
          <Button variant="secondary" className="!px-2 !py-1 text-xs" disabled={page + 1 >= pages} onClick={() => setPage(page + 1)}>
            Next ›
          </Button>
        </div>
      </div>
    </div>
  );
}
