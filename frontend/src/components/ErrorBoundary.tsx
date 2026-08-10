import { Component, type ErrorInfo, type ReactNode } from 'react';

interface Props {
  children: ReactNode;
  /** Optional label for the failing surface (e.g. route name). */
  label?: string;
}

interface State {
  error: Error | null;
}

/**
 * Catches render errors in the subtree (e.g. MapLibre/deck.gl) so the rest of
 * the SPA can still recover via reload.
 */
export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('ErrorBoundary caught', this.props.label || 'app', error, info.componentStack);
  }

  private reset = () => {
    this.setState({ error: null });
  };

  render() {
    const { error } = this.state;
    if (!error) return this.props.children;

    return (
      <div className="page-loading" role="alert" style={{ flexDirection: 'column', gap: 12, padding: 24 }}>
        <strong>Что-то сломалось{this.props.label ? `: ${this.props.label}` : ''}</strong>
        <p style={{ margin: 0, opacity: 0.8, maxWidth: 480, textAlign: 'center' }}>
          {error.message || 'Неизвестная ошибка рендера'}
        </p>
        <div style={{ display: 'flex', gap: 8 }}>
          <button type="button" className="btn" onClick={this.reset}>
            Попробовать снова
          </button>
          <button type="button" className="btn" onClick={() => window.location.reload()}>
            Перезагрузить страницу
          </button>
        </div>
      </div>
    );
  }
}
