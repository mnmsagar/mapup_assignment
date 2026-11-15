import type {
  AlertEventType,
  AlertRule,
  Category,
  Geofence,
  LatLng,
  LiveAlert,
  LocationUpdateResult,
  Vehicle,
  VehicleStatus,
  Violation,
} from './types';

export const API_URL = (import.meta.env.VITE_API_URL ?? 'http://localhost:8080').replace(/\/$/, '');
export const WS_URL = API_URL.replace(/^http/, 'ws') + '/ws/alerts';
/** Optional; must match the backend's API_KEY. Note: anything in VITE_* ships in the bundle. */
const API_KEY: string | undefined = import.meta.env.VITE_API_KEY;

/** Error carrying the backend's message and validation details. */
export class ApiError extends Error {
  constructor(
    message: string,
    public status: number,
    public details: string[] = [],
  ) {
    super(message);
  }

  /** One human-readable line for toasts. */
  get full(): string {
    return this.details.length ? `${this.message}: ${this.details.join('; ')}` : this.message;
  }
}

type Query = Record<string, string | number | undefined | null>;

async function request<T>(method: string, path: string, opts: { body?: unknown; query?: Query } = {}): Promise<T> {
  const url = new URL(API_URL + path);
  for (const [k, v] of Object.entries(opts.query ?? {})) {
    if (v !== undefined && v !== null && v !== '') url.searchParams.set(k, String(v));
  }

  let res: Response;
  try {
    res = await fetch(url, {
      method,
      headers: {
        ...(opts.body ? { 'Content-Type': 'application/json' } : {}),
        ...(API_KEY ? { 'X-API-Key': API_KEY } : {}),
      },
      body: opts.body ? JSON.stringify(opts.body) : undefined,
    });
  } catch {
    throw new ApiError(`Cannot reach the server at ${API_URL}. Is the backend running?`, 0);
  }

  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new ApiError(data.error ?? `Request failed (${res.status})`, res.status, data.details ?? []);
  }
  return data as T;
}

export const api = {
  listGeofences: (category?: Category | '') =>
    request<{ geofences: Geofence[] }>('GET', '/geofences', { query: { category } }).then((r) => r.geofences),

  createGeofence: (body: { name: string; description: string; coordinates: LatLng[]; category: Category }) =>
    request<{ id: string; name: string; status: string }>('POST', '/geofences', { body }),

  listVehicles: () => request<{ vehicles: Vehicle[] }>('GET', '/vehicles').then((r) => r.vehicles),

  createVehicle: (body: { vehicle_number: string; driver_name: string; vehicle_type: string; phone: string }) =>
    request<{ id: string; vehicle_number: string; status: string }>('POST', '/vehicles', { body }),

  updateLocation: (body: { vehicle_id: string; latitude: number; longitude: number; timestamp: string }) =>
    request<LocationUpdateResult>('POST', '/vehicles/location', { body }),

  vehicleStatus: (vehicleId: string) =>
    request<VehicleStatus>('GET', `/vehicles/location/${encodeURIComponent(vehicleId)}`),

  configureAlert: (body: { geofence_id: string; vehicle_id?: string; event_type: AlertEventType }) =>
    request<{ alert_id: string }>('POST', '/alerts/configure', { body }),

  listAlerts: (query: { geofence_id?: string; vehicle_id?: string } = {}) =>
    request<{ alerts: AlertRule[] }>('GET', '/alerts', { query }).then((r) => r.alerts),

  deleteAlert: (alertId: string) => request<{ deleted: boolean }>('DELETE', `/alerts/${encodeURIComponent(alertId)}`),

  recentAlertEvents: (limit = 50) =>
    request<{ events: LiveAlert[] }>('GET', '/alerts/events', { query: { limit } }).then((r) => r.events),

  violationHistory: (query: {
    vehicle_id?: string;
    geofence_id?: string;
    start_date?: string;
    end_date?: string;
    limit?: number;
    offset?: number;
  }) => request<{ violations: Violation[]; total_count: number }>('GET', '/violations/history', { query }),
};

export function errorMessage(err: unknown): string {
  if (err instanceof ApiError) return err.full;
  if (err instanceof Error) return err.message;
  return 'Something went wrong';
}
