import { AlertCircle } from 'lucide-react';

interface ErrorMessageProps {
  message: string;
  onRetry?: () => void;
}

export function ErrorMessage({ message, onRetry }: ErrorMessageProps) {
  return (
    <div className="flex flex-col items-center gap-3 rounded-lg border border-red-200 bg-red-50 px-4 py-8 text-center">
      <AlertCircle className="h-5 w-5 text-red-600" aria-hidden="true" />
      <p className="text-sm text-red-700">{message}</p>
      {onRetry ? (
        <button
          type="button"
          onClick={onRetry}
          className="text-sm font-medium text-red-700 underline underline-offset-2 hover:text-red-800"
        >
          Повторить
        </button>
      ) : null}
    </div>
  );
}
