import type { ReactNode } from 'react';
import { STATUS_LABELS } from '@/api/types';
import type { PaymentStatus } from '@/api/types';

type BadgeVariant = PaymentStatus | 'neutral' | 'info';

interface BadgeProps {
  variant?: BadgeVariant;
  children?: ReactNode;
  className?: string;
}

const VARIANT_CLASSES: Record<BadgeVariant, string> = {
  paid: 'border-green-200 bg-green-50 text-green-700',
  unpaid: 'border-yellow-200 bg-yellow-50 text-yellow-700',
  overdue: 'border-red-200 bg-red-50 text-red-700',
  neutral: 'border-gray-200 bg-gray-50 text-gray-700',
  info: 'border-blue-200 bg-blue-50 text-blue-700',
};

export function Badge({ variant = 'neutral', children, className = '' }: BadgeProps) {
  return (
    <span
      className={[
        'inline-flex items-center gap-1.5 whitespace-nowrap rounded-md border px-2 py-0.5',
        'text-xs font-medium',
        VARIANT_CLASSES[variant],
        className,
      ].join(' ')}
    >
      {children ?? STATUS_LABELS[variant as PaymentStatus] ?? null}
    </span>
  );
}
