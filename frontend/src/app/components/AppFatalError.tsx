import { AlertTriangle, RefreshCw } from 'lucide-react';
import { Button } from '@/components/ui/button';

interface AppFatalErrorProps {
  message: string;
  onRetry: () => void;
}

export default function AppFatalError({ message, onRetry }: AppFatalErrorProps) {
  return (
    <div className="flex min-h-screen items-center justify-center bg-background p-4">
      <div className="w-full max-w-md rounded-2xl border border-destructive/30 bg-card p-6 shadow-sm">
        <div className="mb-4 flex items-center gap-3 text-destructive">
          <AlertTriangle className="h-6 w-6" />
          <h2 className="text-lg font-semibold">应用初始化失败</h2>
        </div>
        <p className="mb-5 text-sm text-muted-foreground">{message}</p>
        <Button onClick={onRetry} className="w-full">
          <RefreshCw className="h-4 w-4" />
          重试
        </Button>
      </div>
    </div>
  );
}
