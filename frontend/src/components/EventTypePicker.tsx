import { EVENT_TYPES, getEventType, isEventTypeId, type EventTypeId } from '../lib/eventTypes'

type Props = {
  /** Empty string means not selected yet (required). */
  value: EventTypeId | ''
  onChange: (id: EventTypeId | '') => void
  disabled?: boolean
  required?: boolean
}

export function EventTypePicker({ value, onChange, disabled, required = true }: Props) {
  const selected = getEventType(value || undefined)
  const Icon = selected?.Icon

  return (
    <label className="block text-sm">
      <span className="mb-1 block text-slate-300">
        Event type {required && <span className="text-red-400">*</span>}
      </span>
      <div className="relative">
        <select
          required={required}
          disabled={disabled}
          value={value}
          onChange={(e) => {
            const v = e.target.value
            onChange(isEventTypeId(v) ? v : '')
          }}
          className="w-full appearance-none rounded-xl border border-white/10 bg-[#111827] py-2.5 pl-3 pr-10 outline-none focus:border-yellow-400 disabled:opacity-60"
          style={selected ? { colorScheme: 'dark' } : { colorScheme: 'dark' }}
        >
          <option value="" disabled>
            Select event type…
          </option>
          {EVENT_TYPES.map((t) => (
            <option key={t.id} value={t.id}>
              {t.label}
            </option>
          ))}
        </select>
        {Icon && (
          <span
            className="pointer-events-none absolute right-9 top-1/2 -translate-y-1/2"
            style={{ color: selected.color }}
          >
            <Icon size={16} />
          </span>
        )}
      </div>
      {selected && <p className="mt-1 text-xs text-slate-500">{selected.description}</p>}
    </label>
  )
}
