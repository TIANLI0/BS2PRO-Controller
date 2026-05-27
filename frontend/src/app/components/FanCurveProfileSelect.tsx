'use client';

import { useMemo } from 'react';
import clsx from 'clsx';
import { useTranslation } from 'react-i18next';
import { Select } from './ui/index';

export type FanCurveProfileOption = {
  id: string;
  name: string;
};

interface FanCurveProfileSelectProps {
  profiles: FanCurveProfileOption[];
  activeProfileId: string;
  onChange: (profileId: string) => void;
  loading?: boolean;
  className?: string;
  placeholder?: string;
}

const EMPTY_PROFILE_SENTINEL = '__no_curve_profile__';

export default function FanCurveProfileSelect({
  profiles,
  activeProfileId,
  onChange,
  loading = false,
  className,
  placeholder,
}: FanCurveProfileSelectProps) {
  const { t } = useTranslation();
  const options = useMemo(
    () => profiles.map((profile) => ({ value: profile.id, label: profile.name })),
    [profiles]
  );

  const resolvedPlaceholder = placeholder || t('fanCurveProfileSelect.placeholder');

  const selectedValue = activeProfileId || options[0]?.value || EMPTY_PROFILE_SENTINEL;

  return (
    <div className={clsx('w-[172px]', className)}>
      <Select
        value={selectedValue}
        onChange={(v: string | number) => {
          const id = String(v);
          if (!id || id === activeProfileId || id === EMPTY_PROFILE_SENTINEL) return;
          onChange(id);
        }}
        options={
          options.length > 0
            ? options
            : [{ value: EMPTY_PROFILE_SENTINEL, label: t('fanCurveProfileSelect.empty'), disabled: true }]
        }
        size="sm"
        placeholder={resolvedPlaceholder}
        disabled={loading || options.length === 0}
        triggerClassName="h-9 rounded-xl border-border/70 bg-background/45 text-[13px]"
      />
    </div>
  );
}
