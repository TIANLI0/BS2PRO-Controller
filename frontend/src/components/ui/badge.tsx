import * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

const badgeVariants = cva("inline-flex items-center rounded-full border font-medium", {
  variants: {
    variant: {
      default: "border-border bg-muted text-muted-foreground",
      success: "border-emerald-300/60 bg-emerald-500/10 text-emerald-600 dark:border-emerald-400/40 dark:text-emerald-300",
      warning: "border-amber-300/60 bg-amber-500/10 text-amber-700 dark:border-amber-400/40 dark:text-amber-300",
      error: "border-red-300/60 bg-red-500/10 text-red-700 dark:border-red-400/40 dark:text-red-300",
      info: "border-primary/30 bg-primary/10 text-primary",
    },
    size: {
      sm: "px-2 py-0.5 text-xs",
      md: "px-2.5 py-1 text-sm",
    },
  },
  defaultVariants: {
    variant: "default",
    size: "sm",
  },
});

export interface BadgeProps extends React.HTMLAttributes<HTMLSpanElement>, VariantProps<typeof badgeVariants> {}

function Badge({ className, variant, size, ...props }: BadgeProps) {
  return <span className={cn(badgeVariants({ variant, size }), className)} {...props} />;
}

export { Badge, badgeVariants };
