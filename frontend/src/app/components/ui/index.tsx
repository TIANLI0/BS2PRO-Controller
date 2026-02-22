'use client';

import React, { Fragment, forwardRef } from 'react';
import { Switch as HeadlessSwitch, Label, Listbox, ListboxButton, ListboxOption, ListboxOptions, Transition } from '@headlessui/react';
import { CheckIcon, ChevronUpDownIcon } from '@heroicons/react/24/outline';
import clsx from 'clsx';

// ==================== Toggle Switch 组件 ====================
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

const colorVariants = {
  blue: 'bg-blue-600 dark:bg-blue-500',
  green: 'bg-green-600 dark:bg-green-500',
  purple: 'bg-purple-600 dark:bg-purple-500',
  orange: 'bg-orange-600 dark:bg-orange-500',
};

const sizeVariants = {
  sm: { switch: 'h-5 w-9', thumb: 'h-4 w-4', translate: 'translate-x-4' },
  md: { switch: 'h-6 w-11', thumb: 'h-5 w-5', translate: 'translate-x-5' },
  lg: { switch: 'h-7 w-14', thumb: 'h-6 w-6', translate: 'translate-x-7' },
};

export const ToggleSwitch = forwardRef<HTMLButtonElement, ToggleSwitchProps>(
  ({ enabled, onChange, disabled = false, loading = false, size = 'md', color = 'blue', label, srLabel }, ref) => {
    const isDisabled = disabled || loading;
    const sizeConfig = sizeVariants[size];

    return (
      <div className="flex items-center gap-3">
        {label && (
          <span className="text-sm font-medium text-gray-700 dark:text-gray-300">{label}</span>
        )}
        <HeadlessSwitch
          ref={ref}
          checked={enabled}
          onChange={onChange}
          disabled={isDisabled}
          className={clsx(
            'group relative inline-flex flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent',
            'transition-all duration-300 ease-out',
            'focus:outline-none focus-visible:ring-2 focus-visible:ring-offset-2',
            'focus-visible:ring-blue-500 focus-visible:ring-offset-white dark:focus-visible:ring-offset-gray-900',
            sizeConfig.switch,
            enabled ? colorVariants[color] : 'bg-gray-200 dark:bg-gray-600',
            isDisabled && 'opacity-50 cursor-not-allowed',
            !isDisabled && 'hover:shadow-lg hover:scale-105',
            loading && 'animate-pulse'
          )}
        >
          <span className="sr-only">{srLabel || label || 'Toggle'}</span>
          <span
            aria-hidden="true"
            className={clsx(
              'pointer-events-none inline-block rounded-full bg-white shadow-lg ring-0',
              'transform transition-all duration-300 ease-out',
              sizeConfig.thumb,
              enabled ? sizeConfig.translate : 'translate-x-0',
              loading && 'opacity-70'
            )}
          >
            {loading && (
              <span className="absolute inset-0 flex items-center justify-center">
                <svg className="animate-spin h-3 w-3 text-gray-400" fill="none" viewBox="0 0 24 24">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                </svg>
              </span>
            )}
          </span>
        </HeadlessSwitch>
      </div>
    );
  }
);
ToggleSwitch.displayName = 'ToggleSwitch';

// ==================== Select 下拉选择组件 ====================
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

const selectSizeVariants = {
  sm: 'py-1.5 pl-3 pr-8 text-sm',
  md: 'py-2 pl-3 pr-10 text-sm',
  lg: 'py-2.5 pl-4 pr-10 text-base',
};

export function Select<T extends string | number>({ 
  value, 
  onChange, 
  options, 
  disabled = false, 
  placeholder = '请选择',
  label,
  size = 'md'
}: SelectProps<T>) {
  const selectedOption = options.find(opt => opt.value === value);

  return (
    <Listbox value={value} onChange={onChange} disabled={disabled}>
      <div className="relative min-w-[100px]">
        {label && (
          <Label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            {label}
          </Label>
        )}
        <ListboxButton
          className={clsx(
            'relative w-full cursor-pointer rounded-lg border',
            'bg-white dark:bg-gray-800 text-left',
            'border-gray-300 dark:border-gray-600',
            'shadow-sm transition-all duration-200',
            'focus:outline-none focus-visible:border-blue-500 focus-visible:ring-2 focus-visible:ring-blue-500/20',
            'hover:border-gray-400 dark:hover:border-gray-500',
            selectSizeVariants[size],
            disabled && 'opacity-50 cursor-not-allowed bg-gray-50 dark:bg-gray-700'
          )}
        >
          <span className={clsx(
            'block truncate pr-6',
            selectedOption ? 'text-gray-900 dark:text-white' : 'text-gray-400 dark:text-gray-500'
          )}>
            {selectedOption?.label || placeholder}
          </span>
          <span className="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-2">
            <ChevronUpDownIcon
              className="h-5 w-5 text-gray-400 transition-transform duration-200"
              aria-hidden="true"
            />
          </span>
        </ListboxButton>

        <Transition
          as={Fragment}
          enter="transition ease-out duration-200"
          enterFrom="opacity-0 translate-y-1"
          enterTo="opacity-100 translate-y-0"
          leave="transition ease-in duration-150"
          leaveFrom="opacity-100 translate-y-0"
          leaveTo="opacity-0 translate-y-1"
        >
          <ListboxOptions
            className={clsx(
              'immersive-select-panel immersive-scrollbar absolute z-50 mt-1 max-h-60 w-full overflow-auto rounded-xl',
              'bg-white dark:bg-gray-800 py-1',
              'border border-gray-200 dark:border-gray-600',
              'shadow-lg ring-1 ring-black/5 dark:ring-white/5',
              'focus:outline-none'
            )}
          >
            {options.map((option) => (
              <ListboxOption
                key={String(option.value)}
                value={option.value}
                disabled={option.disabled}
                className={({ active, selected }) =>
                  clsx(
                    'relative cursor-pointer select-none py-2 px-3 transition-colors duration-100',
                    active && 'bg-blue-50 dark:bg-blue-900/20',
                    selected && 'bg-blue-100 dark:bg-blue-900/30',
                    option.disabled && 'opacity-50 cursor-not-allowed'
                  )
                }
              >
                {({ selected, active }) => (
                  <>
                    <span className={clsx(
                      'block truncate',
                      selected ? 'font-semibold text-blue-600 dark:text-blue-400' : 'font-normal text-gray-900 dark:text-white'
                    )}>
                      {option.label}
                    </span>
                    {option.description && (
                      <span className={clsx(
                        'block truncate text-xs mt-0.5',
                        active ? 'text-blue-600 dark:text-blue-400' : 'text-gray-500 dark:text-gray-400'
                      )}>
                        {option.description}
                      </span>
                    )}
                  </>
                )}
              </ListboxOption>
            ))}
          </ListboxOptions>
        </Transition>
      </div>
    </Listbox>
  );
}

// ==================== Radio Group 组件 ====================
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
  orientation = 'vertical'
}: RadioGroupProps<T>) {
  return (
    <div className="w-full">
      {label && (
        <div className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{label}</div>
      )}
      <div className={clsx(
        'flex gap-2',
        orientation === 'vertical' ? 'flex-col' : 'flex-row flex-wrap'
      )}>
        {options.map((option) => {
          const isSelected = option.value === value;
          const isDisabled = disabled || option.disabled;

          return (
            <button
              key={String(option.value)}
              type="button"
              onClick={() => !isDisabled && onChange(option.value)}
              disabled={isDisabled}
              className={clsx(
                'relative flex items-center rounded-lg px-4 py-3 text-left',
                'border-2 transition-all duration-200',
                'focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2',
                isSelected
                  ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20'
                  : 'border-gray-200 dark:border-gray-600 hover:border-gray-300 dark:hover:border-gray-500 hover:bg-gray-50 dark:hover:bg-gray-700/50',
                isDisabled && 'opacity-50 cursor-not-allowed'
              )}
            >
              <div className={clsx(
                'flex h-5 w-5 items-center justify-center rounded-full border-2 mr-3 flex-shrink-0',
                'transition-all duration-200',
                isSelected
                  ? 'border-blue-500 bg-blue-500'
                  : 'border-gray-300 dark:border-gray-500'
              )}>
                {isSelected && (
                  <div className="h-2 w-2 rounded-full bg-white" />
                )}
              </div>
              <div className="flex-1 min-w-0">
                <div className={clsx(
                  'text-sm font-medium',
                  isSelected ? 'text-blue-900 dark:text-blue-100' : 'text-gray-900 dark:text-white'
                )}>
                  {option.label}
                </div>
                {option.description && (
                  <div className={clsx(
                    'text-xs mt-0.5',
                    isSelected ? 'text-blue-700 dark:text-blue-300' : 'text-gray-500 dark:text-gray-400'
                  )}>
                    {option.description}
                  </div>
                )}
              </div>
            </button>
          );
        })}
      </div>
    </div>
  );
}

// ==================== Slider 滑块组件 ====================
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

export const Slider = forwardRef<HTMLInputElement, SliderProps>(
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
    onChangeEnd
  }, ref) => {
    const percentage = ((value - min) / (max - min)) * 100;

    return (
      <div className="w-full">
        {(label || showValue) && (
          <div className="flex justify-between items-center mb-2">
            {label && <span className="text-sm font-medium text-gray-700 dark:text-gray-300">{label}</span>}
            {showValue && (
              <span className="text-sm font-semibold text-blue-600 dark:text-blue-400">
                {valueFormatter(value)}
              </span>
            )}
          </div>
        )}
        <div className="relative">
          <input
            ref={ref}
            type="range"
            min={min}
            max={max}
            step={step}
            value={value}
            onChange={(e) => onChange(Number(e.target.value))}
            onMouseDown={onChangeStart}
            onTouchStart={onChangeStart}
            onMouseUp={onChangeEnd}
            onTouchEnd={onChangeEnd}
            disabled={disabled}
            className={clsx(
              'w-full h-2 rounded-full appearance-none cursor-pointer',
              'focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2',
              disabled && 'opacity-50 cursor-not-allowed',
              'slider-thumb'
            )}
            style={{
              background: `linear-gradient(to right, #3b82f6 0%, #3b82f6 ${percentage}%, #e5e7eb ${percentage}%, #e5e7eb 100%)`
            }}
          />
        </div>
      </div>
    );
  }
);
Slider.displayName = 'Slider';

// ==================== NumberInput 数字输入组件 ====================
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
      let newValue = Number(e.target.value);
      if (isNaN(newValue)) newValue = min ?? 0;
      if (min !== undefined) newValue = Math.max(min, newValue);
      if (max !== undefined) newValue = Math.min(max, newValue);
      onChange(newValue);
    };

    return (
      <div className="w-full">
        {label && (
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            {label}
          </label>
        )}
        <div className="relative flex items-center">
          <input
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
            className={clsx(
              'w-full px-3 py-2 rounded-lg border text-sm',
              'bg-white dark:bg-gray-800 text-gray-900 dark:text-white',
              'border-gray-300 dark:border-gray-600',
              'focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent',
              'transition-all duration-200',
              disabled && 'opacity-50 cursor-not-allowed bg-gray-50 dark:bg-gray-700',
              suffix && 'pr-12'
            )}
          />
          {suffix && (
            <span className="absolute right-3 text-sm text-gray-500 dark:text-gray-400 pointer-events-none">
              {suffix}
            </span>
          )}
        </div>
      </div>
    );
  }
);
NumberInput.displayName = 'NumberInput';

// ==================== Card 卡片组件 ====================
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
    <div className={clsx(
      'bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700',
      'shadow-sm',
      cardPaddingVariants[padding],
      hover && 'transition-all duration-200 hover:shadow-md hover:border-gray-300 dark:hover:border-gray-600',
      className
    )}>
      {children}
    </div>
  );
}

// ==================== Badge 徽章组件 ====================
interface BadgeProps {
  children: React.ReactNode;
  variant?: 'default' | 'success' | 'warning' | 'error' | 'info';
  size?: 'sm' | 'md';
}

const badgeVariants = {
  default: 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300',
  success: 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400',
  warning: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400',
  error: 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400',
  info: 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400',
};

const badgeSizeVariants = {
  sm: 'px-2 py-0.5 text-xs',
  md: 'px-2.5 py-1 text-sm',
};

export function Badge({ children, variant = 'default', size = 'sm' }: BadgeProps) {
  return (
    <span className={clsx(
      'inline-flex items-center font-medium rounded-full',
      badgeVariants[variant],
      badgeSizeVariants[size]
    )}>
      {children}
    </span>
  );
}

// ==================== Button 按钮组件 ====================
interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary' | 'outline' | 'ghost' | 'danger';
  size?: 'sm' | 'md' | 'lg';
  loading?: boolean;
  icon?: React.ReactNode;
}

const buttonVariants = {
  primary: 'bg-blue-600 text-white hover:bg-blue-700 focus-visible:ring-blue-500 shadow-sm',
  secondary: 'bg-gray-100 text-gray-700 hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-200 dark:hover:bg-gray-600',
  outline: 'border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-700',
  ghost: 'text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700',
  danger: 'bg-red-600 text-white hover:bg-red-700 focus-visible:ring-red-500 shadow-sm',
};

const buttonSizeVariants = {
  sm: 'px-3 py-1.5 text-sm',
  md: 'px-4 py-2 text-sm',
  lg: 'px-6 py-3 text-base',
};

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ variant = 'primary', size = 'md', loading = false, icon, className, children, disabled, ...props }, ref) => {
    return (
      <button
        ref={ref}
        disabled={disabled || loading}
        className={clsx(
          'inline-flex items-center justify-center font-medium rounded-lg',
          'transition-all duration-200',
          'focus:outline-none focus-visible:ring-2 focus-visible:ring-offset-2',
          buttonVariants[variant],
          buttonSizeVariants[size],
          (disabled || loading) && 'opacity-50 cursor-not-allowed',
          className
        )}
        {...props}
      >
        {loading ? (
          <svg className="animate-spin -ml-1 mr-2 h-4 w-4" fill="none" viewBox="0 0 24 24">
            <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
            <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
          </svg>
        ) : icon ? (
          <span className="-ml-0.5 mr-2">{icon}</span>
        ) : null}
        {children}
      </button>
    );
  }
);
Button.displayName = 'Button';
