import { Minus, Plus } from 'lucide-react';
import type { InputHTMLAttributes } from 'react';

export interface NumberStepperProps {
  className?: string;
  disabled?: boolean;
  label?: string;
  max?: number;
  min?: number;
  onChange?: (value: number) => void;
  onCommit?: (value: number) => void;
  step?: number;
  suffix?: string;
  value?: number;
}

export function NumberStepper({ className, disabled, label, max, min, onChange, onCommit, step = 1, suffix, value, ...rest }: NumberStepperProps & Omit<InputHTMLAttributes<HTMLInputElement>, 'disabled' | 'max' | 'min' | 'onChange' | 'step' | 'type' | 'value'>) {
  const clamp = (next: number) => {
    let result = Number.isFinite(next) ? next : min ?? 0;
    if (min != null) result = Math.max(min, result);
    if (max != null) result = Math.min(max, result);
    return result;
  };
  const finiteValue = Number.isFinite(Number(value)) ? Number(value) : undefined;
  const base = finiteValue ?? min ?? 0;
  const commit = (next: number, source: 'input' | 'button' = 'input') => {
    const result = clamp(next);
    onChange?.(result);
    if (source === 'button') onCommit?.(result);
  };
  return (
    <div className={`number-stepper${disabled ? ' is-disabled' : ''}${className ? ` ${className}` : ''}`}>
      <div>
        <button aria-label={label ? `${label} -` : undefined} disabled={disabled || (min != null && base <= min)} type="button" onClick={() => commit(base - step, 'button')}>
          <Minus size={14} />
        </button>
        <input aria-label={label} {...rest} disabled={disabled} max={max} min={min} step={step} type="number" value={finiteValue ?? ''} onChange={(event) => commit(Number(event.target.value))} />
        <button aria-label={label ? `${label} +` : undefined} disabled={disabled || (max != null && base >= max)} type="button" onClick={() => commit(base + step, 'button')}>
          <Plus size={14} />
        </button>
      </div>
      {suffix ? <span>{suffix}</span> : null}
    </div>
  );
}