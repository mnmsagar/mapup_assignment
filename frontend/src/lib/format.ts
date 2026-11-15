import type { Category } from '../api/types';

export const CATEGORY_STYLE: Record<Category, { label: string; color: string; badge: string }> = {
  delivery_zone: { label: 'Delivery zone', color: '#2563eb', badge: 'bg-blue-100 text-blue-800' },
  restricted_zone: { label: 'Restricted zone', color: '#dc2626', badge: 'bg-red-100 text-red-800' },
  toll_zone: { label: 'Toll zone', color: '#d97706', badge: 'bg-amber-100 text-amber-800' },
  customer_area: { label: 'Customer area', color: '#059669', badge: 'bg-emerald-100 text-emerald-800' },
};

export function formatTime(iso: string): string {
  const d = new Date(iso);
  return Number.isNaN(d.getTime())
    ? iso
    : d.toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' });
}

export function relativeTime(iso: string): string {
  const diff = (Date.now() - new Date(iso).getTime()) / 1000;
  if (diff < 0 || Number.isNaN(diff)) return formatTime(iso);
  if (diff < 60) return 'just now';
  if (diff < 3600) return `${Math.floor(diff / 60)} min ago`;
  if (diff < 86400) return `${Math.floor(diff / 3600)} h ago`;
  return formatTime(iso);
}

export const fmtCoord = (n: number) => n.toFixed(5);

/** Value for <input type="datetime-local"> in the user's local time. */
export function toLocalInput(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

/** datetime-local value (local time) -> ISO 8601 UTC, or undefined if empty/invalid. */
export function localInputToISO(v: string): string | undefined {
  if (!v) return undefined;
  const d = new Date(v);
  return Number.isNaN(d.getTime()) ? undefined : d.toISOString();
}
