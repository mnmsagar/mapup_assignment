export type LatLng = [number, number]; // [latitude, longitude]

export const CATEGORIES = ['delivery_zone', 'restricted_zone', 'toll_zone', 'customer_area'] as const;
export type Category = (typeof CATEGORIES)[number];

export const EVENT_TYPES = ['entry', 'exit', 'both'] as const;
export type AlertEventType = (typeof EVENT_TYPES)[number];

export interface Geofence {
  id: string;
  name: string;
  description: string;
  coordinates: LatLng[];
  category: Category;
  status: string;
  created_at: string;
}

export interface VehicleLocation {
  latitude: number;
  longitude: number;
  timestamp: string;
}

export interface Vehicle {
  id: string;
  vehicle_number: string;
  driver_name: string;
  vehicle_type: string;
  phone: string;
  status: string;
  created_at: string;
  current_location: VehicleLocation | null;
}

export interface VehicleStatus {
  vehicle_id: string;
  vehicle_number: string;
  current_location: VehicleLocation | null;
  current_geofences: { geofence_id: string; geofence_name: string; category: Category }[];
}

export interface LocationUpdateResult {
  vehicle_id: string;
  location_updated: boolean;
  current_geofences: { geofence_id: string; geofence_name: string; status: 'inside' }[];
  note?: string;
}

export interface AlertRule {
  alert_id: string;
  geofence_id: string;
  geofence_name: string;
  vehicle_id: string | null;
  vehicle_number: string | null;
  event_type: AlertEventType;
  status: string;
  created_at: string;
}

export interface Violation {
  id: string;
  vehicle_id: string;
  vehicle_number: string;
  geofence_id: string;
  geofence_name: string;
  event_type: 'entry' | 'exit';
  latitude: number;
  longitude: number;
  timestamp: string;
}

/** WebSocket alert message (format fixed by the backend spec). */
export interface LiveAlert {
  event_id: string;
  event_type: 'entry' | 'exit';
  timestamp: string;
  vehicle: { vehicle_id: string; vehicle_number: string; driver_name: string };
  geofence: { geofence_id: string; geofence_name: string; category: Category };
  location: { latitude: number; longitude: number };
}
