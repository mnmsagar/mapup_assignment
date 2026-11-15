import { useEffect, useRef } from 'react';
import { CircleMarker, MapContainer, Polygon, Polyline, Popup, TileLayer, Tooltip, useMap, useMapEvents } from 'react-leaflet';
import L from 'leaflet';
import type { Geofence, LatLng, Vehicle } from '../api/types';
import { CATEGORY_STYLE, fmtCoord, formatTime } from '../lib/format';

export type MapMode = 'idle' | 'draw' | 'pick';

interface Props {
  geofences: Geofence[];
  vehicles: Vehicle[];
  mode: MapMode;
  draftPoints: LatLng[];
  pickedPoint: LatLng | null;
  selectedVehicleId: string | null;
  highlightGeofenceId: string | null;
  onMapClick: (p: LatLng) => void;
  onSelectVehicle: (id: string) => void;
}

const DEFAULT_CENTER: LatLng = [37.7799, -122.4144];

function ClickHandler({ mode, onMapClick }: { mode: MapMode; onMapClick: (p: LatLng) => void }) {
  const map = useMapEvents({
    click(e) {
      if (mode !== 'idle') onMapClick([round(e.latlng.lat), round(e.latlng.lng)]);
    },
  });
  useEffect(() => {
    map.getContainer().style.cursor = mode === 'idle' ? '' : 'crosshair';
  }, [map, mode]);
  return null;
}

/** Fits the view to all geofences/vehicles once, when data first arrives. */
function AutoFit({ geofences, vehicles }: { geofences: Geofence[]; vehicles: Vehicle[] }) {
  const map = useMap();
  const done = useRef(false);
  useEffect(() => {
    if (done.current) return;
    const points: LatLng[] = [
      ...geofences.flatMap((g) => g.coordinates),
      ...vehicles.flatMap((v) => (v.current_location ? [[v.current_location.latitude, v.current_location.longitude] as LatLng] : [])),
    ];
    if (points.length === 0) return;
    map.fitBounds(L.latLngBounds(points), { padding: [40, 40], maxZoom: 15 });
    done.current = true;
  }, [map, geofences, vehicles]);
  return null;
}

const round = (n: number) => Math.round(n * 1e6) / 1e6;

export default function MapView(props: Props) {
  const { geofences, vehicles, mode, draftPoints, pickedPoint, selectedVehicleId, highlightGeofenceId } = props;

  return (
    <div className="relative h-full w-full">
      <MapContainer center={DEFAULT_CENTER} zoom={13} className="h-full w-full" scrollWheelZoom>
        <TileLayer
          attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
          url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
        />
        <ClickHandler mode={mode} onMapClick={props.onMapClick} />
        <AutoFit geofences={geofences} vehicles={vehicles} />

        {geofences.map((g) => {
          const style = CATEGORY_STYLE[g.category];
          const hot = g.id === highlightGeofenceId;
          return (
            <Polygon
              key={g.id}
              positions={g.coordinates}
              // clicks on a polygon bubble up to the map, so drawing/picking inside one still works
              pathOptions={{
                color: style.color,
                weight: hot ? 4 : 2,
                fillOpacity: hot ? 0.4 : 0.15,
                dashArray: g.category === 'restricted_zone' ? '6 4' : undefined,
              }}
            >
              <Tooltip sticky>
                <strong>{g.name}</strong> · {style.label}
              </Tooltip>
              {mode === 'idle' && (
                <Popup>
                  <div className="space-y-1 text-sm">
                    <div className="font-semibold">{g.name}</div>
                    <div className="text-xs text-slate-500">{g.id} · {style.label}</div>
                    {g.description && <div>{g.description}</div>}
                  </div>
                </Popup>
              )}
            </Polygon>
          );
        })}

        {vehicles.map((v) => {
          if (!v.current_location) return null;
          const selected = v.id === selectedVehicleId;
          return (
            <CircleMarker
              key={v.id}
              center={[v.current_location.latitude, v.current_location.longitude]}
              radius={selected ? 10 : 7}
              pathOptions={{ color: '#fff', weight: 2, fillColor: selected ? '#7c3aed' : '#0f172a', fillOpacity: 1 }}
              eventHandlers={{ click: () => props.onSelectVehicle(v.id) }}
            >
              <Tooltip direction="top" offset={[0, -8]} permanent={selected}>
                {v.vehicle_number}
              </Tooltip>
              <Popup>
                <div className="space-y-0.5 text-sm">
                  <div className="font-semibold">{v.vehicle_number}</div>
                  <div>{v.driver_name} · {v.vehicle_type}</div>
                  <div className="text-xs text-slate-500">
                    {fmtCoord(v.current_location.latitude)}, {fmtCoord(v.current_location.longitude)}
                    <br />
                    {formatTime(v.current_location.timestamp)}
                  </div>
                </div>
              </Popup>
            </CircleMarker>
          );
        })}

        {draftPoints.length > 0 && (
          <>
            <Polyline positions={draftPoints} pathOptions={{ color: '#7c3aed', weight: 2, dashArray: '4 4' }} />
            {draftPoints.map((p, i) => (
              <CircleMarker
                key={i}
                center={p}
                radius={i === 0 ? 6 : 4}
                pathOptions={{ color: '#7c3aed', fillColor: i === 0 ? '#7c3aed' : '#fff', fillOpacity: 1 }}
              />
            ))}
          </>
        )}

        {pickedPoint && (
          <CircleMarker center={pickedPoint} radius={9} pathOptions={{ color: '#7c3aed', weight: 3, fillOpacity: 0.2 }}>
            <Tooltip permanent direction="top" offset={[0, -10]}>New location</Tooltip>
          </CircleMarker>
        )}
      </MapContainer>

      {mode !== 'idle' && (
        <div className="pointer-events-none absolute left-1/2 top-3 z-[1000] -translate-x-1/2 rounded-full bg-violet-600 px-4 py-1.5 text-sm font-medium text-white shadow-lg">
          {mode === 'draw' ? `Click the map to add boundary points (${draftPoints.length} so far)` : 'Click the map to set the vehicle location'}
        </div>
      )}

      <Legend />
    </div>
  );
}

function Legend() {
  return (
    <div className="absolute bottom-6 left-3 z-[1000] space-y-1 rounded-md bg-white/95 p-2 text-xs shadow">
      {Object.values(CATEGORY_STYLE).map((s) => (
        <div key={s.label} className="flex items-center gap-2">
          <span className="h-3 w-3 rounded-sm border-2" style={{ borderColor: s.color, background: s.color + '33' }} />
          {s.label}
        </div>
      ))}
      <div className="flex items-center gap-2">
        <span className="h-3 w-3 rounded-full border-2 border-white bg-slate-900 ring-1 ring-slate-300" />
        Vehicle
      </div>
    </div>
  );
}
