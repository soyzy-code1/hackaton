import { Component } from 'react';
import type { ErrorInfo, ReactNode } from 'react';
import { AlertTriangle } from 'lucide-react';

interface ErrorBoundaryProps {
  children: ReactNode;
  /** Changing the key (e.g. the route path) resets the boundary. */
  resetKey?: string;
}

interface ErrorBoundaryState {
  error: Error | null;
}

/**
 * Catches render errors of a single page, so a broken widget shows a local
 * message instead of a blank screen for the whole application.
 */
export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  state: ErrorBoundaryState = { error: null };

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo): void {
    console.error('Page render error:', error, info.componentStack);
  }

  componentDidUpdate(prevProps: ErrorBoundaryProps): void {
    if (prevProps.resetKey !== this.props.resetKey && this.state.error) {
      this.setState({ error: null });
    }
  }

  render(): ReactNode {
    if (!this.state.error) return this.props.children;

    return (
      <div className="card mx-auto mt-6 flex max-w-lg flex-col items-center gap-3 p-8 text-center" role="alert">
        <AlertTriangle className="h-8 w-8 text-red-600" aria-hidden="true" />
        <h2 className="text-base font-semibold text-gray-900">Не удалось отобразить страницу</h2>
        <p className="break-words text-sm text-gray-500 [overflow-wrap:anywhere]">
          {this.state.error.message || 'Произошла непредвиденная ошибка'}
        </p>
        <button
          type="button"
          onClick={() => this.setState({ error: null })}
          className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700"
        >
          Попробовать снова
        </button>
      </div>
    );
  }
}
