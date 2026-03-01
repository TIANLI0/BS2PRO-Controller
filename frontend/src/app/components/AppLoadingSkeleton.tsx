import { Skeleton } from '@/components/ui/skeleton';

export default function AppLoadingSkeleton() {
  return (
    <div className="min-h-screen bg-background">
      {/* Header skeleton */}
      <div className="border-b border-border/50 bg-background/80">
        <div className="mx-auto max-w-[980px] px-5">
          <div className="flex items-center justify-between py-3.5">
            <div className="flex items-center gap-2.5">
              <Skeleton className="h-6 w-32" />
              <Skeleton className="h-6 w-16 rounded-full" />
            </div>
            <div className="flex items-center gap-4">
              <Skeleton className="h-5 w-14" />
              <Skeleton className="h-5 w-20" />
            </div>
          </div>
          <div className="flex gap-0 pb-3 pt-1.5">
            <Skeleton className="mx-6 h-6 flex-1" />
            <Skeleton className="mx-6 h-6 flex-1" />
            <Skeleton className="mx-6 h-6 flex-1" />
          </div>
        </div>
      </div>

      {/* Content skeleton */}
      <div className="mx-auto max-w-[980px] px-5 py-6 space-y-4">
        <Skeleton className="h-24 w-full rounded-2xl" />
        <div className="grid grid-cols-3 gap-4">
          <Skeleton className="h-36 rounded-2xl" />
          <Skeleton className="h-36 rounded-2xl" />
          <Skeleton className="h-36 rounded-2xl" />
        </div>
        <Skeleton className="h-28 w-full rounded-2xl" />
      </div>
    </div>
  );
}
