'use client';

import React, { forwardRef } from 'react';
import clsx from 'clsx';
import { Loader2 } from 'lucide-react';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { Button as ShadcnButton } from '@/components/ui/button';
import { Card as ShadcnCard } from '@/components/ui/card';
import { Badge as ShadcnBadge } from '@/components/ui/badge';
import { Switch } from '@/components/ui/switch';
import { Slider as ShadcnSlider } from '@/components/ui/slider';
import { ScrollArea as ShadcnScrollArea } from '@/components/ui/scroll-area';
import {
  Select as ShadcnSelect,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { RadioGroup as ShadcnRadioGroup, RadioGroupItem } from '@/components/ui/radio-group';

interface ToggleSwitchProps {
  enabled: boolean;
  onChange: (enabled: boolean) => void;
  disabled?: boolean;
  loading?: boolean;
  size?: 'sm' | 'md' | 'lg';
  color?: 'blue' | 'green' | 'purple' | 'orange';
  label?: string;
  srLabel?: string;
}

const toggleColorClasses: Record<NonNullable<ToggleSwitchProps['color']>, string> = {
  blue: 'data-[state=checked]:!bg-primary',
  green: 'data-[state=checked]:!bg-green-600',
  purple: 'data-[state=checked]:!bg-primary',
  orange: 'data-[state=checked]:!bg-orange-600',
};

const toggleSizeClasses: Record<NonNullable<ToggleSwitchProps['size']>, string> = {
  sm: 'h-5 w-9 [&>span]:h-4 [&>span]:w-4 data-[state=checked]:[&>span]:translate-x-4',
  md: 'h-6 w-11 [&>span]:h-5 [&>span]:w-5 data-[state=checked]:[&>span]:translate-x-5',
  lg: 'h-7 w-14 [&>span]:h-6 [&>span]:w-6 data-[state=checked]:[&>span]:translate-x-7',
};

export const ToggleSwitch = forwardRef<HTMLButtonElement, ToggleSwitchProps>(
  ({ enabled, onChange, disabled = false, loading = false, size = 'md', color = 'blue', label, srLabel }, ref) => {
    const isDisabled = disabled || loading;

    return (
      <div className="flex items-center gap-3">
        {label && <span className="text-sm font-medium text-muted-foreground">{label}</span>}
        <Switch
          ref={ref}
          checked={enabled}
          onCheckedChange={onChange}
          disabled={isDisabled}
          aria-label={srLabel || label || 'Toggle'}
          className={clsx(toggleColorClasses[color], toggleSizeClasses[size], loading && 'animate-pulse')}
        />
      </div>
    );
  }
);
ToggleSwitch.displayName = 'ToggleSwitch';

interface SelectOption<T = string> {
  value: T;
  label: string;
  description?: string;
  disabled?: boolean;
}

interface SelectProps<T = string> {
  value: T;
  onChange: (value: T) => void;
  options: SelectOption<T>[];
  disabled?: boolean;
  placeholder?: string;
  label?: string;
  size?: 'sm' | 'md' | 'lg';
}

const selectTriggerSize: Record<'sm' | 'md' | 'lg', string> = {
  sm: 'h-10 text-sm',
  md: 'h-11 text-sm',
  lg: 'h-12 text-base',
};

export function Select<T extends string | number>({
  value,
  onChange,
  options,
  disabled = false,
  placeholder = '请选择',
  label,
  size = 'md',
}: SelectProps<T>) {
  const isNumberValue = typeof value === 'number';

  return (
    <div className="min-w-[120px]">
      {label && <Label className="mb-1 block">{label}</Label>}
      <ShadcnSelect
        value={String(value)}
        onValueChange={(raw) => onChange((isNumberValue ? Number(raw) : raw) as T)}
        disabled={disabled}
      >
        <SelectTrigger
          className={clsx(selectTriggerSize[size], '[&>span]:truncate')}
        >
          <SelectValue placeholder={placeholder} />
        </SelectTrigger>
        <SelectContent>
          {options.map((option) => (
            <SelectItem key={String(option.value)} value={String(option.value)} disabled={option.disabled}>
              {option.label}
            </SelectItem>
          ))}
        </SelectContent>
      </ShadcnSelect>
    </div>
  );
}

interface RadioOption<T = string> {
  value: T;
  label: string;
  description?: string;
  disabled?: boolean;
}

interface RadioGroupProps<T = string> {
  value: T;
  onChange: (value: T) => void;
  options: RadioOption<T>[];
  disabled?: boolean;
  label?: string;
  orientation?: 'horizontal' | 'vertical';
}

export function RadioGroup<T extends string | number>({
  value,
  onChange,
  options,
  disabled = false,
  label,
  orientation = 'vertical',
}: RadioGroupProps<T>) {
  const isNumberValue = typeof value === 'number';

  return (
    <div className="w-full">
      {label && <div className="mb-2 text-sm font-medium text-muted-foreground">{label}</div>}
      <ShadcnRadioGroup
        value={String(value)}
        onValueChange={(raw) => onChange((isNumberValue ? Number(raw) : raw) as T)}
        className={clsx('gap-2', orientation === 'horizontal' ? 'grid-flow-col auto-cols-fr' : 'grid-cols-1')}
        disabled={disabled}
      >
        {options.map((option) => {
          const selected = option.value === value;
          const itemDisabled = disabled || option.disabled;
          return (
            <label
              key={String(option.value)}
              className={clsx(
                'flex cursor-pointer items-center rounded-lg border-2 px-4 py-3 transition-all',
                selected
                  ? 'border-primary/50 bg-primary/10'
                  : 'border-border hover:border-primary/30 hover:bg-muted/70',
                itemDisabled && 'cursor-not-allowed opacity-50'
              )}
            >
              <RadioGroupItem value={String(option.value)} disabled={itemDisabled} className="mr-3" />
              <div className="min-w-0 flex-1">
                <div className={clsx('text-sm font-medium', selected ? 'text-primary' : 'text-foreground')}>
                  {option.label}
                </div>
                {option.description && (
                  <div className={clsx('mt-0.5 text-xs', selected ? 'text-primary/80' : 'text-muted-foreground')}>
                    {option.description}
                  </div>
                )}
              </div>
            </label>
          );
        })}
      </ShadcnRadioGroup>
    </div>
  );
}

interface SliderProps {
  value: number;
  onChange: (value: number) => void;
  min: number;
  max: number;
  step?: number;
  disabled?: boolean;
  label?: string;
  showValue?: boolean;
  valueFormatter?: (value: number) => string;
  onChangeStart?: () => void;
  onChangeEnd?: () => void;
}

export const Slider = forwardRef<React.ElementRef<typeof ShadcnSlider>, SliderProps>(
  ({
    value,
    onChange,
    min,
    max,
    step = 1,
    disabled = false,
    label,
    showValue = true,
    valueFormatter = (v) => String(v),
    onChangeStart,
    onChangeEnd,
  }, ref) => {
    return (
      <div className="w-full">
        {(label || showValue) && (
          <div className="mb-2 flex items-center justify-between">
            {label && <span className="text-sm font-medium text-muted-foreground">{label}</span>}
            {showValue && <span className="text-sm font-semibold text-primary">{valueFormatter(value)}</span>}
          </div>
        )}
        <ShadcnSlider
          ref={ref}
          min={min}
          max={max}
          step={step}
          value={[value]}
          onValueChange={(next) => onChange(next[0] ?? value)}
          onPointerDown={onChangeStart}
          onPointerUp={onChangeEnd}
          disabled={disabled}
          className={clsx(
            'w-full',
            disabled && 'opacity-50'
          )}
        />
      </div>
    );
  }
);
Slider.displayName = 'Slider';

interface ScrollAreaProps extends React.ComponentPropsWithoutRef<typeof ShadcnScrollArea> {
  children: React.ReactNode;
}

export function ScrollArea({ children, className, ...props }: ScrollAreaProps) {
  return (
    <ShadcnScrollArea className={className} {...props}>
      {children}
    </ShadcnScrollArea>
  );
}

interface NumberInputProps {
  value: number;
  onChange: (value: number) => void;
  min?: number;
  max?: number;
  step?: number;
  disabled?: boolean;
  label?: string;
  suffix?: string;
  onFocus?: () => void;
  onBlur?: () => void;
}

export const NumberInput = forwardRef<HTMLInputElement, NumberInputProps>(
  ({ value, onChange, min, max, step = 1, disabled = false, label, suffix, onFocus, onBlur }, ref) => {
    const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
      let nextValue = Number(e.target.value);
      if (Number.isNaN(nextValue)) nextValue = min ?? 0;
      if (min !== undefined) nextValue = Math.max(min, nextValue);
      if (max !== undefined) nextValue = Math.min(max, nextValue);
      onChange(nextValue);
    };

    return (
      <div className="w-full">
        {label && <Label className="mb-1 block">{label}</Label>}
        <div className="relative flex items-center">
          <Input
            ref={ref}
            type="number"
            value={value}
            onChange={handleChange}
            onFocus={onFocus}
            onBlur={onBlur}
            min={min}
            max={max}
            step={step}
            disabled={disabled}
            className={clsx(suffix && 'pr-12')}
          />
          {suffix && <span className="pointer-events-none absolute right-3 text-sm text-muted-foreground">{suffix}</span>}
        </div>
      </div>
    );
  }
);
NumberInput.displayName = 'NumberInput';

interface CardProps {
  children: React.ReactNode;
  className?: string;
  padding?: 'none' | 'sm' | 'md' | 'lg';
  hover?: boolean;
}

const cardPaddingVariants = {
  none: '',
  sm: 'p-3',
  md: 'p-4',
  lg: 'p-6',
};

export function Card({ children, className, padding = 'md', hover = false }: CardProps) {
  return (
    <ShadcnCard
      className={clsx(
        cardPaddingVariants[padding],
        hover && 'transition-all duration-200 hover:-translate-y-0.5 hover:border-primary/30 hover:shadow-md',
        className
      )}
    >
      {children}
    </ShadcnCard>
  );
}

interface BadgeProps {
  children: React.ReactNode;
  variant?: 'default' | 'success' | 'warning' | 'error' | 'info';
  size?: 'sm' | 'md';
}

export function Badge({ children, variant = 'default', size = 'sm' }: BadgeProps) {
  return (
    <ShadcnBadge variant={variant} size={size}>
      {children}
    </ShadcnBadge>
  );
}

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary' | 'outline' | 'ghost' | 'danger';
  size?: 'sm' | 'md' | 'lg';
  loading?: boolean;
  icon?: React.ReactNode;
}

const buttonVariantMap: Record<NonNullable<ButtonProps['variant']>, 'default' | 'secondary' | 'outline' | 'ghost' | 'destructive'> = {
  primary: 'default',
  secondary: 'secondary',
  outline: 'outline',
  ghost: 'ghost',
  danger: 'destructive',
};

const buttonSizeMap: Record<NonNullable<ButtonProps['size']>, 'sm' | 'default' | 'lg'> = {
  sm: 'sm',
  md: 'default',
  lg: 'lg',
};

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ variant = 'primary', size = 'md', loading = false, icon, className, children, disabled, ...props }, ref) => {
    return (
      <ShadcnButton
        ref={ref}
        variant={buttonVariantMap[variant]}
        size={buttonSizeMap[size]}
        disabled={disabled || loading}
        className={className}
        {...props}
      >
        {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : icon ? <span>{icon}</span> : null}
        {children}
      </ShadcnButton>
    );
  }
);
Button.displayName = 'Button';

export { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
export {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog';
export { Skeleton } from '@/components/ui/skeleton';
